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
	ReasonForce   = "force"
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
	Row    store.VendorPricing
	Exists bool // the row is in the database
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

func ptrEq(a, b *float64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func copyPtr(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// load reads everything and computes every vendor. With no error text it also takes over
// vendors that have no row yet and stores the computed state. Callers hold s.mu.
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
	canPersist := len(sn.errs) == 0
	for _, v := range vendors {
		vs := &vendorState{V: v}
		vs.Row, vs.Exists = rowOf[v.ID]
		if !vs.Exists {
			vs.Row = sn.takeover(v)
		}
		if t, ok := totals[v.Name]; ok {
			vs.Rev, vs.Cost = t.Revenue, t.Cost
		}
		sn.evaluate(vs)
		sn.vs = append(sn.vs, vs)
		sn.byID[v.ID] = vs
	}
	if canPersist {
		for _, vs := range sn.vs {
			if err := s.persist(ctx, vs); err != nil {
				return nil, err
			}
		}
	}
	return sn, nil
}

// takeover builds the row of a vendor nam has not priced yet from what NewAPI has: an
// existing GroupRatio becomes a fixed price that reproduces it exactly (no price change).
func (sn *snap) takeover(v store.Vendor) store.VendorPricing {
	row := store.VendorPricing{VendorID: v.ID, Mode: store.PricingFixed, State: store.PricingUnset, UpdatedAt: time.Now().Unix()}
	if r, ok := sn.opts.GroupRatio[v.GroupName]; ok && finitePositive(r) {
		price := PriceFromRatio(r, sn.usd, sn.site)
		ratio := r
		row.SellPrice, row.LastWrittenPrice, row.LastWrittenRatio = &price, copyPtr(&price), &ratio
	}
	return row
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
		Mode: vs.Row.Mode, SellPrice: vs.Row.SellPrice, Markup: vs.Row.Markup, Ref: vs.Ref,
		USDRate: sn.usd, SiteRate: sn.site, ProtectLimit: sn.protect, MinChange: sn.minChange,
		NewAPIRatio: vs.NewAPI, LastPrice: vs.Row.LastWrittenPrice, LastRatio: vs.Row.LastWrittenRatio,
	})
}

// persist stores the computed state of a vendor (and creates its row on first sight).
func (s *Service) persist(ctx context.Context, vs *vendorState) error {
	var refCost *float64
	if vs.Ref != nil {
		refCost = copyPtr(&vs.Ref.Cost)
	}
	if vs.Exists && vs.Row.State == vs.Res.State && ptrEq(vs.Row.PendingPrice, vs.Res.Pending) && ptrEq(vs.Row.ReferenceCost, refCost) {
		return nil
	}
	vs.Row.State, vs.Row.PendingPrice, vs.Row.ReferenceCost = vs.Res.State, copyPtr(vs.Res.Pending), refCost
	vs.Row.UpdatedAt = time.Now().Unix()
	if err := s.St.DB.WithContext(ctx).Save(&vs.Row).Error; err != nil {
		return err
	}
	vs.Exists = true
	return nil
}

// saveRow stores vs.Row after the caller changed it, recomputing the state first.
func (s *Service) saveRow(ctx context.Context, sn *snap, vs *vendorState) error {
	sn.evaluate(vs)
	vs.Exists = false // force the write
	return s.persist(ctx, vs)
}

// Item is one vendor in GET /vendor-pricing.
type Item struct {
	VendorID         uint       `json:"vendor_id"`
	Vendor           string     `json:"vendor"`
	Group            string     `json:"group"`
	Order            int        `json:"order"`
	Mode             string     `json:"mode"`
	Markup           *float64   `json:"markup"`
	SellPrice        *float64   `json:"sell_price"`
	Reference        *Reference `json:"reference"`
	TargetPrice      *float64   `json:"target_price"`
	TargetRatio      *float64   `json:"target_ratio"`
	NewAPIRatio      *float64   `json:"newapi_ratio"`
	LastWrittenPrice *float64   `json:"last_written_price"`
	LastWrittenRatio *float64   `json:"last_written_ratio"`
	LastWrittenAt    *int64     `json:"last_written_at"`
	PendingPrice     *float64   `json:"pending_price"`
	Change           *float64   `json:"change"`
	SmallChange      bool       `json:"small_change"`
	State            string     `json:"state"`
	Revenue7d        float64    `json:"revenue_7d"`
	Cost7d           float64    `json:"cost_7d"`
	Margin7d         *float64   `json:"margin_7d"`
}

func (vs *vendorState) item() Item {
	it := Item{
		VendorID: vs.V.ID, Vendor: vs.V.Name, Group: vs.V.GroupName, Order: vs.V.Order, Mode: vs.Row.Mode,
		Markup: vs.Row.Markup, SellPrice: vs.Row.SellPrice, Reference: vs.Ref,
		TargetPrice: vs.Res.Target, TargetRatio: vs.Res.TargetRatio, NewAPIRatio: vs.NewAPI,
		LastWrittenPrice: vs.Row.LastWrittenPrice, LastWrittenRatio: vs.Row.LastWrittenRatio,
		PendingPrice: vs.Res.Pending, Change: vs.Res.Change, SmallChange: vs.Res.SmallChange,
		State: vs.Res.State, Revenue7d: vs.Rev, Cost7d: vs.Cost,
	}
	if vs.Row.LastWrittenAt > 0 {
		at := vs.Row.LastWrittenAt
		it.LastWrittenAt = &at
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

// Overview computes every vendor (taking over vendors that have no row yet).
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
// GroupRatio edits the vendor group ratio directly (fixed mode only, not together with
// SellPrice): it is rounded to 3 decimals and converted to the sell price that produces it.
type UpdateInput struct {
	Mode       *string  `json:"mode"`
	SellPrice  *float64 `json:"sell_price"`
	Markup     *float64 `json:"markup"`
	GroupRatio *float64 `json:"group_ratio"`
}

func (in UpdateInput) validate() error {
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
	Mode      string   `json:"mode"`
	SellPrice *float64 `json:"sell_price"`
	Markup    *float64 `json:"markup"`
}

// Update changes a vendor's mode, sell price, markup or (fixed mode) group ratio. Writes nam
// only; clears pending_price.
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
	if !vs.Exists {
		return nil, resp.Conflict("NewAPI 选项或设置异常，暂时不能编辑厂商定价：" + sn.errText())
	}
	before := rowDoc{vs.Row.Mode, vs.Row.SellPrice, vs.Row.Markup}
	if in.GroupRatio != nil {
		mode := vs.Row.Mode
		if in.Mode != nil {
			mode = *in.Mode
		}
		if mode != store.PricingFixed {
			return nil, resp.Invalid("跟随模式的倍率由参考成本决定，请先改为固定模式再直接修改厂商分组倍率")
		}
		if !finitePositive(sn.usd) || !finitePositive(sn.site) {
			return nil, resp.Conflict("美元参考汇率或站点汇率无效，不能换算厂商分组倍率")
		}
		// Round the ratio first, then convert without rounding so that the target ratio is
		// exactly what the user typed.
		price := PriceFromRatio(Round(*in.GroupRatio, RatioPlaces), sn.usd, sn.site)
		if !(finitePositive(price) && price <= 10) {
			return nil, resp.Invalid("该倍率折合的售价必须大于 0 且不超过 10（原厂价的倍数）")
		}
		in.SellPrice = &price
	} else if in.SellPrice != nil {
		if _, err := GroupRatio(*in.SellPrice, sn.usd, sn.site); err != nil {
			return nil, resp.Invalid(err.Error())
		}
	}
	if in.Mode != nil {
		vs.Row.Mode = *in.Mode
	}
	if in.SellPrice != nil {
		vs.Row.SellPrice = copyPtr(in.SellPrice)
	}
	if in.Markup != nil {
		vs.Row.Markup = copyPtr(in.Markup)
	}
	vs.Row.PendingPrice = nil
	if err := s.saveRow(ctx, sn, vs); err != nil {
		return nil, err
	}
	s.record(ctx, store.AuditLog{Kind: store.AuditPrice, Target: fmt.Sprintf("nam:vendor_pricing:%d", id), Action: "update",
		Summary: "修改厂商定价 " + vs.V.Name, Before: audit.JSON(before), After: audit.JSON(rowDoc{vs.Row.Mode, vs.Row.SellPrice, vs.Row.Markup})})
	it := vs.item()
	return &it, nil
}

// Adopt takes NewAPI's current ratio as the vendor's fixed price (drift only). Writes nam only.
func (s *Service) Adopt(ctx context.Context, id uint) (*Item, error) {
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
	if !vs.Exists {
		return nil, resp.Conflict("NewAPI 选项或设置异常，暂时不能采纳：" + sn.errText())
	}
	if vs.Res.State != store.PricingDrift {
		return nil, resp.Conflict(fmt.Sprintf("厂商 %s 当前不是漂移状态，不能采纳 NewAPI 值", vs.V.Name))
	}
	if vs.NewAPI == nil || !finitePositive(*vs.NewAPI) {
		return nil, resp.Conflict(fmt.Sprintf("NewAPI 中没有厂商 %s 的有效倍率，无法采纳；可以选择以 nam 为准覆盖", vs.V.Name))
	}
	before := rowDoc{vs.Row.Mode, vs.Row.SellPrice, vs.Row.Markup}
	ratio := *vs.NewAPI
	price := PriceFromRatio(ratio, sn.usd, sn.site)
	vs.Row.Mode = store.PricingFixed
	vs.Row.SellPrice, vs.Row.LastWrittenPrice, vs.Row.LastWrittenRatio = &price, copyPtr(&price), &ratio
	vs.Row.PendingPrice = nil
	if err := s.saveRow(ctx, sn, vs); err != nil {
		return nil, err
	}
	s.record(ctx, store.AuditLog{Kind: store.AuditPrice, Target: fmt.Sprintf("nam:vendor_pricing:%d", id), Action: "adopt",
		Summary: "采纳 NewAPI 倍率作为厂商 " + vs.V.Name + " 的固定售价", Before: audit.JSON(before),
		After: audit.JSON(rowDoc{vs.Row.Mode, vs.Row.SellPrice, vs.Row.Markup})})
	it := vs.item()
	return &it, nil
}

// Job is the scheduled pricing job: recompute every vendor (persisting 待确认 / 漂移), then,
// when pricing_auto_write is on, write the follow-mode vendors that are 待写入 and not a
// small change.
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
	msg := fmt.Sprintf("%d 个厂商；写入 %d；待确认 %d；漂移 %d；无可用渠道 %d",
		len(sn.vs), written, count[store.PricingPending], count[store.PricingDrift], count[store.PricingNoChannel])
	if writeErr != nil {
		return msg, fmt.Errorf("自动写入失败：%w", writeErr)
	}
	return msg, nil
}
