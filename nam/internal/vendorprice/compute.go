// Package vendorprice computes vendor sell prices and writes the resulting group ratios to
// NewAPI (GroupRatio, UserUsableGroups, AutoGroups).
//
// Prices are multiples of the official price (0.30 = 30%). compute.go is pure: no I/O.
package vendorprice

import (
	"errors"
	"math"
	"sort"

	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

const (
	// DriftTolerance: NewAPI's ratio differs from the last written one by more than this = 漂移.
	DriftTolerance = 1e-6
	// RatioPlaces is the number of decimals of a written group ratio.
	RatioPlaces = 3
	// SyncTolerance: a ratio is "already in NewAPI" when it is within half a write step
	// (ratios are written with 3 decimals, so 5e-4), plus a tiny margin against float noise at
	// the boundary. This also makes taking over a NewAPI ratio that is not a 3-decimal number
	// (e.g. 0.2857) count as synced: taking over must never change a price.
	SyncTolerance = 5e-4 + 1e-9
)

// Round rounds x to the given number of decimal places.
func Round(x float64, places int) float64 {
	p := math.Pow10(places)
	return math.Round(x*p) / p
}

func finitePositive(x float64) bool { return x > 0 && !math.IsInf(x, 0) && !math.IsNaN(x) }

// ChannelInput is one candidate channel for the reference cost.
type ChannelInput struct {
	ID          int
	Name        string
	Status      int
	Priority    int64
	Weight      uint
	ChannelCost *float64 // 上游渠道售价
}

// RefChannel is a channel that contributed to a reference cost.
type RefChannel struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Weight      uint    `json:"weight"`
	ChannelCost float64 `json:"channel_cost"`
}

// Reference is the 参考成本 of a vendor: weighted average 上游渠道售价 of its top priority layer.
type Reference struct {
	Cost     float64      `json:"cost"`
	Priority int64        `json:"priority"`
	Channels []RefChannel `json:"channels"`
}

// ReferenceCost picks the enabled channels that have an upstream price, keeps the layer with
// the largest priority and averages their prices weighted by weight (simple average when
// every weight is 0). Returns nil when no channel qualifies.
func ReferenceCost(chs []ChannelInput) *Reference {
	var cand []ChannelInput
	for _, c := range chs {
		if c.Status != ports.ChannelEnabled || c.ChannelCost == nil || !finitePositive(*c.ChannelCost) {
			continue
		}
		cand = append(cand, c)
	}
	if len(cand) == 0 {
		return nil
	}
	top := cand[0].Priority
	for _, c := range cand {
		if c.Priority > top {
			top = c.Priority
		}
	}
	var layer []ChannelInput
	for _, c := range cand {
		if c.Priority == top {
			layer = append(layer, c)
		}
	}
	sort.Slice(layer, func(i, j int) bool { return layer[i].ID < layer[j].ID })
	var sumW, sumWC, sumC float64
	ref := &Reference{Priority: top, Channels: make([]RefChannel, 0, len(layer))}
	for _, c := range layer {
		w := float64(c.Weight)
		sumW += w
		sumWC += w * *c.ChannelCost
		sumC += *c.ChannelCost
		ref.Channels = append(ref.Channels, RefChannel{ID: c.ID, Name: c.Name, Weight: c.Weight, ChannelCost: *c.ChannelCost})
	}
	if sumW > 0 {
		ref.Cost = sumWC / sumW
	} else {
		ref.Cost = sumC / float64(len(layer))
	}
	return ref
}

// TargetSellPrice is the sell price a vendor should have: the fixed price, or in follow mode
// 参考成本 × 加价比例 rounded to 4 decimals. Nil when it cannot be computed.
func TargetSellPrice(mode string, sellPrice, markup *float64, ref *Reference) *float64 {
	var v float64
	switch mode {
	case store.PricingFollow:
		if markup == nil || ref == nil {
			return nil
		}
		v = Round(ref.Cost**markup, 4)
	default:
		if sellPrice == nil {
			return nil
		}
		v = *sellPrice
	}
	if !finitePositive(v) {
		return nil
	}
	return &v
}

// GroupRatio converts a sell price (multiple of the official price) to the NewAPI group
// ratio: sell × 美元参考汇率 ÷ 站点汇率, rounded to 3 decimals. A sell price so small that
// the ratio rounds to 0 is an error (a ratio of 0 is never written).
func GroupRatio(sell, usdRate, siteRate float64) (float64, error) {
	if !finitePositive(sell) || !finitePositive(usdRate) || !finitePositive(siteRate) {
		return 0, errors.New("售价、美元参考汇率和站点汇率必须是大于 0 的有限数")
	}
	r := Round(sell*usdRate/siteRate, RatioPlaces)
	if !finitePositive(r) {
		return 0, ErrRatioRoundsToZero
	}
	return r, nil
}

// ErrRatioRoundsToZero is returned by GroupRatio when the ratio rounds to 0 at 3 decimals.
var ErrRatioRoundsToZero = errors.New("厂商分组倍率舍入到 3 位小数后为 0（售价过小）")

// PriceFromRatio is the inverse of GroupRatio: ratio × 站点汇率 ÷ 美元参考汇率. It does no
// rounding, so adopting an existing ratio (or a ratio typed by the user) keeps it exactly.
func PriceFromRatio(ratio, usdRate, siteRate float64) float64 { return ratio * siteRate / usdRate }

// Facts are the inputs of Classify.
type Facts struct {
	Mode         string
	Configured   bool // fixed: sell price set; follow: markup set
	HasRef       bool // a reference cost exists (follow mode)
	Target       *float64
	TargetRatio  *float64
	NewAPIRatio  *float64 // current GroupRatio of the vendor group in NewAPI
	LastRatio    *float64 // GroupRatio nam wrote last
	Change       *float64 // |target − last price| ÷ last price
	ProtectLimit float64  // 变化保护阈值
}

// Classify returns the state: 未设置 → 无可用渠道 → 漂移 → 待确认 → 待写入 → 已同步.
func Classify(f Facts) string {
	switch {
	case !f.Configured:
		return store.PricingUnset
	case f.Mode == store.PricingFollow && !f.HasRef:
		return store.PricingNoChannel
	case f.Target == nil || f.TargetRatio == nil:
		return store.PricingUnset
	case f.LastRatio != nil && (f.NewAPIRatio == nil || math.Abs(*f.NewAPIRatio-*f.LastRatio) > DriftTolerance):
		return store.PricingDrift
	case f.Mode == store.PricingFollow && f.Change != nil && *f.Change > f.ProtectLimit:
		return store.PricingPending
	case f.NewAPIRatio == nil || math.Abs(*f.NewAPIRatio-*f.TargetRatio) >= SyncTolerance:
		return store.PricingToWrite
	}
	return store.PricingSynced
}

// Input is everything Evaluate needs about one vendor.
type Input struct {
	Mode                 string
	SellPrice, Markup    *float64
	Ref                  *Reference
	USDRate, SiteRate    float64
	ProtectLimit         float64 // 变化保护阈值
	MinChange            float64 // 最小变动
	NewAPIRatio          *float64
	LastPrice, LastRatio *float64
}

// Result is the computed state of one vendor.
type Result struct {
	Target       *float64
	TargetRatio  *float64
	Change       *float64 // absolute relative change vs the last written price
	SignedChange *float64
	SmallChange  bool
	Pending      *float64 // pending price (state pending only)
	State        string
}

// Evaluate computes target price/ratio, change, small-change flag and state.
func Evaluate(in Input) Result {
	var r Result
	configured := in.SellPrice != nil
	if in.Mode == store.PricingFollow {
		configured = in.Markup != nil
	}
	r.Target = TargetSellPrice(in.Mode, in.SellPrice, in.Markup, in.Ref)
	if r.Target != nil {
		if ratio, err := GroupRatio(*r.Target, in.USDRate, in.SiteRate); err == nil {
			r.TargetRatio = &ratio
		}
	}
	if r.Target != nil && in.LastPrice != nil && *in.LastPrice > 0 {
		signed := (*r.Target - *in.LastPrice) / *in.LastPrice
		abs := math.Abs(signed)
		r.SignedChange, r.Change = &signed, &abs
		r.SmallChange = abs < in.MinChange
	}
	r.State = Classify(Facts{
		Mode: in.Mode, Configured: configured, HasRef: in.Ref != nil,
		Target: r.Target, TargetRatio: r.TargetRatio,
		NewAPIRatio: in.NewAPIRatio, LastRatio: in.LastRatio,
		Change: r.Change, ProtectLimit: in.ProtectLimit,
	})
	if r.State == store.PricingPending {
		p := *r.Target
		r.Pending = &p
	}
	return r
}
