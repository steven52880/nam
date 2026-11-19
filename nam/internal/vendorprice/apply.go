package vendorprice

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/steven52880/nam/internal/audit"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"gorm.io/gorm"
)

// ApplyInput is the body of preview and apply.
//
// VendorIDs nil (omitted) = every 待写入 vendor; a non-nil list = exactly those vendors.
// IncludeGroups nil = true.
type ApplyInput struct {
	VendorIDs     []uint `json:"vendor_ids"`
	IncludeGroups *bool  `json:"include_groups"`
}

// PlanVendor is one vendor whose ratio will be (re)written.
type PlanVendor struct {
	VendorID    uint     `json:"vendor_id"`
	Vendor      string   `json:"vendor"`
	Group       string   `json:"group"`
	BeforePrice *float64 `json:"before_price"`
	AfterPrice  *float64 `json:"after_price"`
	BeforeRatio *float64 `json:"before_ratio"`
	AfterRatio  *float64 `json:"after_ratio"`
}

// Skipped is a vendor that is left alone, with the reason.
type Skipped struct {
	VendorID uint   `json:"vendor_id"`
	Vendor   string `json:"vendor"`
	State    string `json:"state"`
	Reason   string `json:"reason"`
}

// OptionChange is one NewAPI option that changes (raw JSON strings).
type OptionChange struct {
	Key    string `json:"key"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// PreviewResult is POST /vendor-pricing/preview.
type PreviewResult struct {
	Vendors []PlanVendor   `json:"vendors"`
	Skipped []Skipped      `json:"skipped"`
	Options []OptionChange `json:"options"`
}

// ApplyResult is POST /vendor-pricing/apply (and confirm).
type ApplyResult struct {
	Written []string     `json:"written"`
	Vendors []PlanVendor `json:"vendors"`
	Skipped []Skipped    `json:"skipped"`
}

type plan struct {
	sel     []*vendorState
	reason  map[uint]string
	vendors []PlanVendor
	skipped []Skipped
	keys    []OptionChange // changed options, in write order
	want    Options        // options as they must be in NewAPI after the write
}

func (p *plan) changes(key string) bool {
	for _, k := range p.keys {
		if k.Key == key {
			return true
		}
	}
	return false
}

// matches reports whether raw, parsed, equals the wanted value of key.
func (p *plan) matches(key, raw string) bool {
	switch key {
	case KeyGroupRatio:
		got, err := ParseGroupRatio(raw)
		return err == nil && EqualGroupRatio(got, p.want.GroupRatio)
	case KeyUserUsableGroups:
		got, err := ParseUserUsableGroups(raw)
		return err == nil && EqualUserUsableGroups(got, p.want.UserUsableGroups)
	case KeyAutoGroups:
		got, err := ParseAutoGroups(raw)
		return err == nil && EqualAutoGroups(got, p.want.AutoGroups)
	}
	return false
}

func pct(v float64) string {
	p := Round(v*100, 1)
	s := strconv.FormatFloat(p, 'f', -1, 64)
	if p >= 0 {
		s = "+" + s
	}
	return s + "%"
}

func skipReason(vs *vendorState) string {
	switch vs.Res.State {
	case store.PricingPending:
		if vs.Res.Change != nil {
			return fmt.Sprintf("变化 %s，超过变化保护阈值", pct(*vs.Res.Change))
		}
		return "变化超过变化保护阈值"
	case store.PricingUnset:
		if vs.Row.Mode == store.PricingFollow && vs.Row.Markup != nil {
			return "售价过小，厂商分组倍率舍入到 3 位小数后为 0，不能写入"
		}
		return "还没有设置售价或加价比例"
	case store.PricingNoChannel:
		return "该分组下没有启用且有上游渠道售价的渠道，算不出参考成本"
	case store.PricingSynced:
		return "已是最新，无需写入"
	}
	return "当前状态不能写入"
}

// buildPlan decides which vendors and options are written. It never writes.
func (s *Service) buildPlan(sn *snap, in ApplyInput, trigger string, confirmID uint) (*plan, error) {
	if msg := sn.errText(); msg != "" {
		return nil, resp.Conflict("NewAPI 选项或设置异常，不能写入：" + msg)
	}
	p := &plan{reason: map[uint]string{}, vendors: []PlanVendor{}, skipped: []Skipped{}}
	skip := func(vs *vendorState) {
		p.skipped = append(p.skipped, Skipped{VendorID: vs.V.ID, Vendor: vs.V.Name, State: vs.Res.State, Reason: skipReason(vs)})
	}
	pick := func(vs *vendorState, reason string) {
		p.sel = append(p.sel, vs)
		p.reason[vs.V.ID] = reason
	}
	autoOr := func(r string) string {
		if trigger == store.TriggerAuto {
			return ReasonAuto
		}
		return r
	}

	if confirmID != 0 {
		vs, ok := sn.byID[confirmID]
		if !ok {
			return nil, store.ErrNotFound
		}
		if vs.Res.State != store.PricingPending {
			return nil, resp.Conflict(fmt.Sprintf("厂商 %s 当前不是待确认状态，无需确认", vs.V.Name))
		}
		pick(vs, ReasonConfirm)
	} else {
		explicit := in.VendorIDs != nil
		listed := map[uint]bool{}
		for _, id := range in.VendorIDs {
			if _, ok := sn.byID[id]; !ok {
				return nil, resp.Invalid(fmt.Sprintf("厂商 %d 不存在", id))
			}
			listed[id] = true
		}
		for _, vs := range sn.vs {
			if explicit && !listed[vs.V.ID] {
				continue
			}
			switch vs.Res.State {
			case store.PricingToWrite:
				pick(vs, autoOr(ReasonManual))
			case store.PricingSynced:
				if explicit {
					skip(vs)
				}
			default:
				skip(vs)
			}
		}
	}

	set := map[string]float64{}
	for _, vs := range p.sel {
		set[vs.V.GroupName] = *vs.Res.TargetRatio
		p.vendors = append(p.vendors, PlanVendor{
			VendorID: vs.V.ID, Vendor: vs.V.Name, Group: vs.V.GroupName,
			BeforePrice: vs.currentPrice(sn), AfterPrice: copyPtr(vs.Res.Target),
			BeforeRatio: copyPtr(vs.NewAPI), AfterRatio: copyPtr(vs.Res.TargetRatio),
		})
	}
	p.want = Options{
		GroupRatio:       MergeGroupRatio(sn.opts.GroupRatio, set),
		UserUsableGroups: sn.opts.UserUsableGroups,
		AutoGroups:       sn.opts.AutoGroups,
	}
	if !EqualGroupRatio(sn.opts.GroupRatio, p.want.GroupRatio) {
		p.keys = append(p.keys, OptionChange{Key: KeyGroupRatio, Before: sn.raw[KeyGroupRatio], After: MarshalGroupRatio(p.want.GroupRatio)})
	}
	if in.IncludeGroups == nil || *in.IncludeGroups {
		refs := sn.refs()
		if u := MergeUserUsableGroups(sn.opts.UserUsableGroups, refs); !EqualUserUsableGroups(sn.opts.UserUsableGroups, u) {
			p.want.UserUsableGroups = u
			p.keys = append(p.keys, OptionChange{Key: KeyUserUsableGroups, Before: sn.raw[KeyUserUsableGroups], After: MarshalUserUsableGroups(u)})
		}
		if a := MergeAutoGroups(sn.opts.AutoGroups, refs); !EqualAutoGroups(sn.opts.AutoGroups, a) {
			p.want.AutoGroups = a
			p.keys = append(p.keys, OptionChange{Key: KeyAutoGroups, Before: sn.raw[KeyAutoGroups], After: MarshalAutoGroups(a)})
		}
	}
	return p, nil
}

// Preview shows what apply would do for the same input.
func (s *Service) Preview(ctx context.Context, in ApplyInput) (*PreviewResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sn, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	p, err := s.buildPlan(sn, in, store.TriggerManual, 0)
	if err != nil {
		return nil, err
	}
	opts := p.keys
	if opts == nil {
		opts = []OptionChange{}
	}
	return &PreviewResult{Vendors: p.vendors, Skipped: p.skipped, Options: opts}, nil
}

// Apply writes the planned vendors and options to NewAPI. trigger is store.TriggerManual or
// store.TriggerAuto (audit and price_history reason).
func (s *Service) Apply(ctx context.Context, in ApplyInput, trigger string) (*ApplyResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sn, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	return s.applyLocked(ctx, sn, in, trigger, 0)
}

// Confirm writes the pending price of one 待确认 vendor (protection bypassed).
func (s *Service) Confirm(ctx context.Context, id uint) (*ApplyResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sn, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	return s.applyLocked(ctx, sn, ApplyInput{}, store.TriggerManual, id)
}

func kindOf(key string) string {
	if key == KeyGroupRatio {
		return store.AuditPrice
	}
	return store.AuditSetting
}

// applyLocked is the shared write flow of Apply, Confirm and Job. Callers hold s.mu.
//
//  1. (load) the three options are read and parsed; a parse error stops everything.
//  2. the plan computes new values (every ratio is finite and > 0).
//  3. only the changed options are written: GroupRatio → UserUsableGroups → AutoGroups.
//  4. what was written is read back and compared by parsed value.
//  5. each vendor whose ratio changed gets a price_history row; vendor_pricing is updated.
func (s *Service) applyLocked(ctx context.Context, sn *snap, in ApplyInput, trigger string, confirmID uint) (*ApplyResult, error) {
	ctx = audit.WithTrigger(ctx, trigger)
	p, err := s.buildPlan(sn, in, trigger, confirmID)
	if err != nil {
		return nil, err
	}
	res := &ApplyResult{Written: []string{}, Vendors: p.vendors, Skipped: p.skipped}

	var failure error
	for _, k := range p.keys {
		if err := s.Options.Set(ctx, k.Key, k.After); err != nil {
			failure = resp.Upstream(fmt.Errorf("写入 NewAPI 选项 %s 失败：%w", k.Key, err))
			break
		}
		res.Written = append(res.Written, k.Key)
	}
	verified, verr := s.verify(ctx, p, res.Written)
	if failure == nil {
		failure = verr
	}
	// A GroupRatio that is not in NewAPI must not be recorded as written.
	if !p.changes(KeyGroupRatio) || verified[KeyGroupRatio] {
		if err := s.finish(ctx, sn, p, p.changes(KeyGroupRatio)); err != nil && failure == nil {
			failure = err
		}
	}
	if failure != nil {
		return nil, failure
	}
	return res, nil
}

// verify reads the written options back and compares them by parsed value.
func (s *Service) verify(ctx context.Context, p *plan, written []string) (map[string]bool, error) {
	ok := map[string]bool{}
	if len(written) == 0 {
		return ok, nil
	}
	got, err := s.Options.Get(ctx, written...)
	if err != nil {
		return ok, resp.Upstream(fmt.Errorf("写入后读回 NewAPI 选项失败：%w", err))
	}
	var bad []string
	for _, k := range written {
		raw, present := got[k]
		if present && p.matches(k, raw) {
			ok[k] = true
			continue
		}
		bad = append(bad, k)
		var want string
		for _, c := range p.keys {
			if c.Key == k {
				want = c.After
			}
		}
		s.record(ctx, store.AuditLog{Kind: kindOf(k), Target: "option:" + k, Action: "verify",
			Summary: "写入后核对 " + k + "：NewAPI 中的值与写入值不一致", Before: want, After: raw,
			Result: store.ResultFail, Error: "写入后读回的值与写入值不一致"})
	}
	if len(bad) > 0 {
		return ok, resp.Upstream(fmt.Errorf("写入后核对失败：NewAPI 中的 %s 与写入值不一致", strings.Join(bad, "、")))
	}
	return ok, nil
}

type historyDoc struct {
	Mode       string   `json:"mode"`
	SellPrice  *float64 `json:"sell_price"`
	GroupRatio *float64 `json:"group_ratio"`
}

// currentPrice is the sell price NewAPI's current ratio stands for (nil when NewAPI has none).
func (vs *vendorState) currentPrice(sn *snap) *float64 {
	if vs.NewAPI == nil || !finitePositive(*vs.NewAPI) || !finitePositive(sn.usd) || !finitePositive(sn.site) {
		return nil
	}
	p := PriceFromRatio(*vs.NewAPI, sn.usd, sn.site)
	return &p
}

// finish records the written vendors: price_history for every vendor whose ratio changed, and
// clears the draft ratio of the written fixed-mode vendors.
func (s *Service) finish(ctx context.Context, sn *snap, p *plan, wroteRatio bool) error {
	if len(p.sel) == 0 {
		return nil
	}
	now := time.Now().Unix()
	var auditID uint
	if wroteRatio {
		var ids []uint
		s.St.DB.WithContext(ctx).Model(&store.AuditLog{}).Where("target = ?", "option:"+KeyGroupRatio).
			Order("id DESC").Limit(1).Pluck("id", &ids)
		if len(ids) > 0 {
			auditID = ids[0]
		}
	}
	return s.St.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, vs := range p.sel {
			price, ratio := *vs.Res.Target, *vs.Res.TargetRatio
			if vs.NewAPI == nil || *vs.NewAPI != ratio {
				id := vs.V.ID
				h := store.PriceHistory{
					At: now, Key: fmt.Sprintf("vendor:%d", id), VendorID: &id, Reason: p.reason[id], AuditID: auditID,
					Before: audit.JSON(historyDoc{Mode: vs.Row.Mode, SellPrice: vs.currentPrice(sn), GroupRatio: vs.NewAPI}),
					After:  audit.JSON(historyDoc{Mode: vs.Row.Mode, SellPrice: &price, GroupRatio: &ratio}),
				}
				if err := tx.Create(&h).Error; err != nil {
					return err
				}
			}
			if vs.Exists && vs.Row.DraftRatio != nil {
				vs.Row.DraftRatio, vs.Row.UpdatedAt = nil, now
				if err := tx.Save(&vs.Row).Error; err != nil {
					return err
				}
			}
			vs.NewAPI = copyPtr(&ratio)
			sn.evaluate(vs)
		}
		return nil
	})
}
