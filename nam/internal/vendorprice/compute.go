// Package vendorprice computes vendor sell prices and writes the resulting group ratios to
// NewAPI (GroupRatio, UserUsableGroups, AutoGroups).
//
// Prices are multiples of the official price (0.30 = 30%). compute.go is pure: no I/O.
//
// nam does not keep what NewAPI already has. Fixed mode reads NewAPI's GroupRatio (plus an
// optional draft ratio that is not written yet); follow mode computes the price from the
// reference cost and overwrites whatever is in NewAPI (nam wins).
package vendorprice

import (
	"errors"
	"math"
	"sort"

	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

const (
	// RatioPlaces is the number of decimals of a written group ratio.
	RatioPlaces = 3
	// SyncTolerance: a ratio is "already in NewAPI" when it is within half a write step
	// (ratios are written with 3 decimals, so 5e-4), plus a tiny margin against float noise at
	// the boundary. So a NewAPI ratio that is not a 3-decimal number (e.g. 0.2857) counts as
	// synced with its rounded value 0.286.
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

// FollowPrice is the sell price of a follow-mode vendor: 参考成本 × 加价比例 rounded to 4
// decimals. Nil when it cannot be computed.
func FollowPrice(markup *float64, ref *Reference) *float64 {
	if markup == nil || ref == nil {
		return nil
	}
	v := Round(ref.Cost**markup, 4)
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
// rounding, so a ratio read from NewAPI (or typed by the user) is kept exactly.
func PriceFromRatio(ratio, usdRate, siteRate float64) float64 { return ratio * siteRate / usdRate }

// Input is everything Evaluate needs about one vendor.
type Input struct {
	Mode              string
	Markup            *float64 // follow mode
	DraftRatio        *float64 // fixed mode: change not written yet
	Ref               *Reference
	USDRate, SiteRate float64
	ProtectLimit      float64 // 变化保护阈值
	MinChange         float64 // 最小变动
	NewAPIRatio       *float64
}

// Result is the computed state of one vendor.
type Result struct {
	Target      *float64 // 目标售价
	TargetRatio *float64 // 目标倍率
	Change      *float64 // (目标售价 − NewAPI 当前售价) ÷ NewAPI 当前售价，带方向；NewAPI 没有该分组时 nil
	SmallChange bool     // follow mode: |Change| < 最小变动
	State       string
}

// Evaluate computes target price/ratio, change, small-change flag and state.
//
// Fixed: target ratio = draft ?? NewAPI's ratio; nothing at all → unset; a draft that differs
// from NewAPI → to_write; otherwise synced.
// Follow: unset (no markup) → no_channel (no reference cost) → pending (|change| above the
// protection limit) → to_write (target ratio differs from NewAPI) → synced.
func Evaluate(in Input) Result {
	var r Result
	ratesOK := finitePositive(in.USDRate) && finitePositive(in.SiteRate)
	var newapi *float64
	if in.NewAPIRatio != nil && finitePositive(*in.NewAPIRatio) {
		v := *in.NewAPIRatio
		newapi = &v
	}

	if in.Mode == store.PricingFollow {
		switch {
		case in.Markup == nil:
			r.State = store.PricingUnset
			return r
		case in.Ref == nil:
			r.State = store.PricingNoChannel
			return r
		}
		r.Target = FollowPrice(in.Markup, in.Ref)
		if r.Target != nil && ratesOK {
			if ratio, err := GroupRatio(*r.Target, in.USDRate, in.SiteRate); err == nil {
				r.TargetRatio = &ratio
			}
		}
	} else {
		var ratio *float64
		switch {
		case in.DraftRatio != nil && finitePositive(*in.DraftRatio):
			v := *in.DraftRatio
			ratio = &v
		case newapi != nil:
			ratio = newapi
		}
		if ratio != nil && ratesOK {
			price := PriceFromRatio(*ratio, in.USDRate, in.SiteRate)
			r.Target, r.TargetRatio = &price, ratio
		}
	}
	if r.Target == nil || r.TargetRatio == nil {
		r.State = store.PricingUnset
		return r
	}

	var abs float64
	if newapi != nil && ratesOK {
		cur := PriceFromRatio(*newapi, in.USDRate, in.SiteRate)
		signed := (*r.Target - cur) / cur
		abs = math.Abs(signed)
		r.Change = &signed
		r.SmallChange = in.Mode == store.PricingFollow && abs < in.MinChange
	}
	switch {
	case in.Mode == store.PricingFollow && r.Change != nil && abs > in.ProtectLimit:
		r.State = store.PricingPending
	case newapi == nil || math.Abs(*newapi-*r.TargetRatio) >= SyncTolerance:
		r.State = store.PricingToWrite
	default:
		r.State = store.PricingSynced
	}
	return r
}
