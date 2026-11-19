package vendorprice

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/steven52880/nam/internal/audit"
	"github.com/steven52880/nam/internal/channels"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/stats"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

// Service computes vendor prices and writes them to NewAPI.
//
// Options must be the audit-wrapped OptionStore. One mutex serializes everything that reads
// or writes vendor_pricing or NewAPI options, so the scheduled job and a manual write never
// run at the same time.
type Service struct {
	St       *store.Store
	Options  ports.OptionStore
	Channels *channels.Service
	Audit    *audit.Recorder

	mu sync.Mutex
}

// History reasons.
const (
	ReasonManual  = "manual"
	ReasonAuto    = "auto"
	ReasonConfirm = "confirm"
)

// Defaults for settings that are missing or invalid.
const (
	defaultProtect   = 0.5
	defaultMinChange = 0.02
	revenueDays      = 7
)

// vendorState is one vendor with its row, reference cost and computed result.
type vendorState struct {
	V      store.Vendor
	Row    store.VendorPricing // zero value (fixed mode, no draft) when the vendor has no row
	Exists bool                // the row is in the database
	Ref    *Reference
	Res    Result
	NewAPI *float64 // GroupRatio[group] in NewAPI right now
	Rev    float64
	Cost   float64
}

// snap is one consistent read of everything pricing depends on.
type snap struct {
	site, usd, protect, minChange float64
	autoWrite                     bool
	raw                           map[string]string // option key → raw value as read
	opts                          Options
	errs                          []string
	vs                            []*vendorState
	byID                          map[uint]*vendorState
	chans                         []channels.LiveChannel
}

func (sn *snap) errText() string { return strings.Join(sn.errs, "；") }

func (sn *snap) refs() []VendorRef {
	out := make([]VendorRef, 0, len(sn.vs))
	for _, v := range sn.vs {
		out = append(out, VendorRef{Name: v.V.Name, Group: v.V.GroupName})
	}
	return out
}

func (s *Service) record(ctx context.Context, e store.AuditLog) uint {
	if s.Audit == nil {
		return 0
	}
	return s.Audit.Record(ctx, e)
}

func inGroup(groups, want string) bool {
	for _, g := range strings.Split(groups, ",") {
		if strings.TrimSpace(g) == want {
			return true
		}
	}
	return false
}

func copyPtr(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// load reads everything and computes every vendor. It never writes. Callers hold s.mu.
func (s *Service) load(ctx context.Context) (*snap, error) {
	sn := &snap{byID: map[uint]*vendorState{}}
	sn.site = s.St.GetFloat(ctx, store.KeySiteRate, 7)
	sn.usd = s.St.USDRate(ctx)
	sn.protect = s.St.GetFloat(ctx, store.KeyProtectThreshold, defaultProtect)
	if !finitePositive(sn.protect) {
		sn.protect = defaultProtect
	}
	sn.minChange = s.St.GetFloat(ctx, store.KeyPricingMinChange, defaultMinChange)
	if sn.minChange < 0 || sn.minChange >= 1 || math.IsNaN(sn.minChange) {
		sn.minChange = defaultMinChange
	}
	sn.autoWrite = s.St.GetBool(ctx, store.KeyPricingAutoWrite, false)
	sn.opts = Options{GroupRatio: map[string]float64{}, UserUsableGroups: map[string]string{}, AutoGroups: []string{}}
	if !finitePositive(sn.site) {
		sn.errs = append(sn.errs, "站点汇率无效，请检查设置 site_rate")
	}

	raw, err := s.Options.Get(ctx, optionOrder...)
	if err != nil {
		sn.errs = append(sn.errs, "读取 NewAPI 选项失败："+err.Error())
	} else {
		var perr map[string]error
		sn.raw = raw
		sn.opts, perr = ParseOptions(raw)
		for _, k := range optionOrder {
			if e := perr[k]; e != nil {
				sn.errs = append(sn.errs, e.Error())
			}
		}
	}
	if sn.chans, err = s.Channels.LiveChannels(ctx); err != nil {
		return nil, err
	}
	totals, err := stats.VendorTotals(ctx, s.St, revenueDays)
	if err != nil {
		return nil, err
	}

	db := s.St.DB.WithContext(ctx)
	var vendors []store.Vendor
	if err := db.Order("`order`, id").Find(&vendors).Error; err != nil {
		return nil, err
	}
	var rows []store.VendorPricing
	if err := db.Find(&rows).Error; err != nil {
		return nil, err
	}
	rowOf := make(map[uint]store.VendorPricing, len(rows))
	for _, r := range rows {
		rowOf[r.VendorID] = r
	}
	for _, v := range vendors {
		vs := &vendorState{V: v}
		vs.Row, vs.Exists = rowOf[v.ID]
		if !vs.Exists {
			vs.Row = store.VendorPricing{VendorID: v.ID}
		}
		if vs.Row.Mode != store.PricingFollow {
			vs.Row.Mode = store.PricingFixed
		}
		if t, ok := totals[v.Name]; ok {
			vs.Rev, vs.Cost = t.Revenue, t.Cost
		}
		sn.evaluate(vs)
		sn.vs = append(sn.vs, vs)
		sn.byID[v.ID] = vs
	}
	return sn, nil
}

// evaluate fills Ref, NewAPI and Res of one vendor.
func (sn *snap) evaluate(vs *vendorState) {
	var in []ChannelInput
	for _, c := range sn.chans {
		if inGroup(c.Group, vs.V.GroupName) {
			in = append(in, ChannelInput{ID: c.ID, Name: c.Name, Status: c.Status, Priority: c.Priority, Weight: c.Weight, ChannelCost: c.ChannelCost})
		}
	}
	vs.Ref = ReferenceCost(in)
	vs.NewAPI = nil
	if r, ok := sn.opts.GroupRatio[vs.V.GroupName]; ok {
		vs.NewAPI = &r
	}
	vs.Res = Evaluate(Input{
		Mode: vs.Row.Mode, Markup: vs.Row.Markup, DraftRatio: vs.Row.DraftRatio, Ref: vs.Ref,
		USDRate: sn.usd, SiteRate: sn.site, ProtectLimit: sn.protect, MinChange: sn.minChange,
		NewAPIRatio: vs.NewAPI,
	})
}

// Item is one vendor in GET /vendor-pricing.
type Item struct {
	VendorID    uint       `json:"vendor_id"`
	Vendor      string     `json:"vendor"`
	Group       string     `json:"group"`
	Order       int        `json:"order"`
	Mode        string     `json:"mode"`
	Markup      *float64   `json:"markup"`
	SellPrice   *float64   `json:"sell_price"`
	Reference   *Reference `json:"reference"`
	TargetPrice *float64   `json:"target_price"`
	TargetRatio *float64   `json:"target_ratio"`
	NewAPIRatio *float64   `json:"newapi_ratio"`
	DraftRatio  *float64   `json:"draft_ratio"`
	Change      *float64   `json:"change"`
	SmallChange bool       `json:"small_change"`
	State       string     `json:"state"`
	Revenue7d   float64    `json:"revenue_7d"`
	Cost7d      float64    `json:"cost_7d"`
	Margin7d    *float64   `json:"margin_7d"`
}

func (vs *vendorState) item() Item {
	it := Item{
		VendorID: vs.V.ID, Vendor: vs.V.Name, Group: vs.V.GroupName, Order: vs.V.Order, Mode: vs.Row.Mode,
		Markup: copyPtr(vs.Row.Markup), SellPrice: copyPtr(vs.Res.Target), Reference: vs.Ref,
		TargetPrice: copyPtr(vs.Res.Target), TargetRatio: copyPtr(vs.Res.TargetRatio), NewAPIRatio: copyPtr(vs.NewAPI),
		Change: copyPtr(vs.Res.Change), SmallChange: vs.Res.SmallChange,
		State: vs.Res.State, Revenue7d: vs.Rev, Cost7d: vs.Cost,
	}
	// A draft that already equals NewAPI is not a pending edit.
	if vs.Row.Mode == store.PricingFixed && vs.Res.State != store.PricingSynced {
		it.DraftRatio = copyPtr(vs.Row.DraftRatio)
	}
	if vs.Rev > 0 {
		m := (vs.Rev - vs.Cost) / vs.Rev
		it.Margin7d = &m
	}
	return it
}

// GroupsInfo shows how AutoGroups and UserUsableGroups differ from what nam wants.
type GroupsInfo struct {
	AutoGroups struct {
		Current []string `json:"current"`
		Target  []string `json:"target"`
	} `json:"auto_groups"`
	UserUsableGroups struct {
		Current map[string]string `json:"current"`
		Target  map[string]string `json:"target"`
	} `json:"user_usable_groups"`
	Changed bool `json:"changed"`
}

// OverviewResult is GET /vendor-pricing.
type OverviewResult struct {
	SiteRate         float64    `json:"site_rate"`
	USDRate          float64    `json:"usd_rate"`
	AutoWrite        bool       `json:"auto_write"`
	ProtectThreshold float64    `json:"protect_threshold"`
	MinChange        float64    `json:"min_change"`
	Items            []Item     `json:"items"`
	Groups           GroupsInfo `json:"groups"`
	Error            string     `json:"error"`
}

func (sn *snap) overview() *OverviewResult {
	out := &OverviewResult{
		SiteRate: sn.site, USDRate: sn.usd, AutoWrite: sn.autoWrite, ProtectThreshold: sn.protect, MinChange: sn.minChange,
		Items: make([]Item, 0, len(sn.vs)), Error: sn.errText(),
	}
	for _, vs := range sn.vs {
		out.Items = append(out.Items, vs.item())
	}
	g := &out.Groups
	g.AutoGroups.Current, g.AutoGroups.Target = sn.opts.AutoGroups, MergeAutoGroups(sn.opts.AutoGroups, sn.refs())
	g.UserUsableGroups.Current, g.UserUsableGroups.Target = sn.opts.UserUsableGroups, MergeUserUsableGroups(sn.opts.UserUsableGroups, sn.refs())
	if out.Error == "" {
		g.Changed = !EqualAutoGroups(g.AutoGroups.Current, g.AutoGroups.Target) ||
			!EqualUserUsableGroups(g.UserUsableGroups.Current, g.UserUsableGroups.Target)
	}
	return out
}

// Overview computes every vendor. It never writes the database.
func (s *Service) Overview(ctx context.Context) (*OverviewResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sn, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	return sn.overview(), nil
}

// UpdateInput is PUT /vendor-pricing/:id. Nil fields are left unchanged.
//
// SellPrice and GroupRatio (fixed mode only, not together) become a draft ratio: nothing is
// written to NewAPI until preview/apply. DiscardDraft clears the draft and cannot be combined
// with the other fields.
type UpdateInput struct {
	Mode         *string  `json:"mode"`
	SellPrice    *float64 `json:"sell_price"`
	Markup       *float64 `json:"markup"`
	GroupRatio   *float64 `json:"group_ratio"`
	DiscardDraft bool     `json:"discard_draft"`
}

func (in UpdateInput) validate() error {
	if in.DiscardDraft && (in.Mode != nil || in.SellPrice != nil || in.Markup != nil || in.GroupRatio != nil) {
		return resp.Invalid("撤销修改（discard_draft）不能和其它字段同时提交")
	}
	if in.Mode != nil && *in.Mode != store.PricingFixed && *in.Mode != store.PricingFollow {
		return resp.Invalid("模式只能是 fixed 或 follow")
	}
	if in.SellPrice != nil && !(finitePositive(*in.SellPrice) && *in.SellPrice <= 10) {
		return resp.Invalid("售价必须大于 0 且不超过 10（原厂价的倍数，0.3 = 30%）")
	}
	if in.Markup != nil && !(finitePositive(*in.Markup) && *in.Markup <= 100) {
		return resp.Invalid("加价比例必须大于 0 且不超过 100")
	}
	if in.GroupRatio != nil {
		if in.SellPrice != nil {
			return resp.Invalid("不能同时修改售价和厂商分组倍率，请只给其中一个")
		}
		if !finitePositive(*in.GroupRatio) {
			return resp.Invalid("厂商分组倍率必须大于 0")
		}
		if Round(*in.GroupRatio, RatioPlaces) <= 0 {
			return resp.Invalid("厂商分组倍率舍入到 3 位小数后为 0，请填写不小于 0.001 的值")
		}
	}
	return nil
}

type rowDoc struct {
	Mode       string   `json:"mode"`
	Markup     *float64 `json:"markup"`
	DraftRatio *float64 `json:"draft_ratio"`
}

func docOf(r store.VendorPricing) rowDoc {
	return rowDoc{Mode: r.Mode, Markup: copyPtr(r.Markup), DraftRatio: copyPtr(r.DraftRatio)}
}

// Update changes a vendor's mode, markup or (fixed mode) draft ratio. Writes nam only.
func (s *Service) Update(ctx context.Context, id uint, in UpdateInput) (*Item, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sn, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	vs, ok := sn.byID[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	if msg := sn.errText(); msg != "" {
		return nil, resp.Conflict("NewAPI 选项或设置异常，暂时不能编辑厂商定价：" + msg)
	}
	before := docOf(vs.Row)
	row := vs.Row
	row.DraftRatio = copyPtr(row.DraftRatio)
	row.Markup = copyPtr(row.Markup)
	if in.Mode != nil {
		row.Mode = *in.Mode
	}
	if in.Markup != nil {
		row.Markup = copyPtr(in.Markup)
	}

	var draft *float64 // the ratio asked for (fixed mode)
	switch {
	case in.GroupRatio != nil:
		r := Round(*in.GroupRatio, RatioPlaces)
		draft = &r
		if !finitePositive(sn.usd) || !finitePositive(sn.site) {
			return nil, resp.Conflict("美元参考汇率或站点汇率无效，不能换算厂商分组倍率")
		}
		if price := PriceFromRatio(r, sn.usd, sn.site); !(finitePositive(price) && price <= 10) {
			return nil, resp.Invalid("该倍率折合的售价必须大于 0 且不超过 10（原厂价的倍数）")
		}
	case in.SellPrice != nil:
		r, err := GroupRatio(*in.SellPrice, sn.usd, sn.site)
		if err != nil {
			return nil, resp.Invalid(err.Error())
		}
		draft = &r
	}
	if draft != nil && row.Mode != store.PricingFixed {
		return nil, resp.Invalid("跟随模式的倍率由参考成本决定，请先改为固定模式再修改售价或厂商分组倍率")
	}

	switch {
	case row.Mode == store.PricingFollow, in.DiscardDraft:
		row.DraftRatio = nil
	case draft != nil:
		if vs.NewAPI != nil && finitePositive(*vs.NewAPI) && math.Abs(*vs.NewAPI-*draft) < SyncTolerance {
			row.DraftRatio = nil // same as what NewAPI has: nothing to write
		} else {
			row.DraftRatio = draft
		}
	}
	row.UpdatedAt = time.Now().Unix()
	if err := s.St.DB.WithContext(ctx).Save(&row).Error; err != nil {
		return nil, err
	}
	vs.Row, vs.Exists = row, true
	sn.evaluate(vs)
	s.record(ctx, store.AuditLog{Kind: store.AuditPrice, Target: fmt.Sprintf("nam:vendor_pricing:%d", id), Action: "update",
		Summary: "修改厂商定价 " + vs.V.Name, Before: audit.JSON(before), After: audit.JSON(docOf(vs.Row))})
	it := vs.item()
	return &it, nil
}

// Job is the scheduled pricing job: recompute every vendor, then, when pricing_auto_write is
// on, write the follow-mode vendors that are 待写入 and not a small change. Fixed-mode drafts
// are never written by the job.
func (s *Service) Job(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx = audit.WithTrigger(ctx, store.TriggerAuto)
	sn, err := s.load(ctx)
	if err != nil {
		return "", err
	}
	if msg := sn.errText(); msg != "" {
		return "", errors.New("无法计算定价：" + msg)
	}
	written := 0
	var writeErr error
	if sn.autoWrite {
		var ids []uint
		for _, vs := range sn.vs {
			if vs.Row.Mode == store.PricingFollow && vs.Res.State == store.PricingToWrite && !vs.Res.SmallChange {
				ids = append(ids, vs.V.ID)
			}
		}
		if len(ids) > 0 {
			var res *ApplyResult
			if res, writeErr = s.applyLocked(ctx, sn, ApplyInput{VendorIDs: ids}, store.TriggerAuto, 0); writeErr == nil {
				written = len(res.Vendors)
			}
		}
	}
	count := map[string]int{}
	for _, vs := range sn.vs {
		count[vs.Res.State]++
	}
	msg := fmt.Sprintf("%d 个厂商；写入 %d；待确认 %d；无可用渠道 %d",
		len(sn.vs), written, count[store.PricingPending], count[store.PricingNoChannel])
	if writeErr != nil {
		return msg, fmt.Errorf("自动写入失败：%w", writeErr)
	}
	return msg, nil
}
