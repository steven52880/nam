package pricing

import (
	"context"
	"encoding/json"
	"math"

	"github.com/steven52880/nam/internal/store"
)

// ModelDeviationThreshold flags a model whose own 上游价 ÷ 原厂价 differs from the channel's
// baseline (上游分组倍率 when known, else the channel median) by more than this fraction.
const ModelDeviationThreshold = 0.01

// Channel-level discount sources (ChannelRate.Source).
const (
	SourceManual = "manual" // 主人手填 (channel_ext.manual_discount)
	SourceAuto   = "auto"   // 渠道各模型 上游价 ÷ 原厂价 的中位数
	SourceGroup  = "group"  // 令牌所在分组的上游分组倍率
	SourceNone   = "none"   // 都没有
)

// Model-level discount sources (ModelRate.Source).
const (
	ModelOverride = "override" // 模型级覆盖
	ModelManual   = "manual"   // 渠道手填值
	ModelOwn      = "model"    // 这个模型自己的 上游价 ÷ 原厂价
	ModelChannel  = "channel"  // 渠道级倍率（auto / group）
	ModelNone     = "none"     // 算不出
)

type groupRef struct {
	upstream uint
	group    string
}

// Resolver turns cached upstream prices and group ratios into discounts. It reads nam.db
// once (LoadResolver) and answers every later question from memory; it never calls HTTP.
type Resolver struct {
	book   *Book
	prices map[groupRef]map[string]UpstreamPrice
	groups map[groupRef]float64
}

// NewResolver indexes upstream_model_price and upstream_group rows.
func NewResolver(book *Book, prices []store.UpstreamModelPrice, groups []store.UpstreamGroup) *Resolver {
	r := &Resolver{book: book, prices: map[groupRef]map[string]UpstreamPrice{}, groups: map[groupRef]float64{}}
	for _, p := range prices {
		ref := groupRef{p.UpstreamID, p.Group}
		m := r.prices[ref]
		if m == nil {
			m = map[string]UpstreamPrice{}
			r.prices[ref] = m
		}
		m[p.Model] = UpstreamPrice{Input: p.Input, PerRequest: p.PerRequest}
	}
	for _, g := range groups {
		r.groups[groupRef{g.UpstreamID, g.Name}] = g.Ratio
	}
	return r
}

// LoadResolver loads the whole price and group cache in two queries.
func LoadResolver(ctx context.Context, st *store.Store, book *Book) (*Resolver, error) {
	var prices []store.UpstreamModelPrice
	var groups []store.UpstreamGroup
	db := st.DB.WithContext(ctx)
	if err := db.Find(&prices).Error; err != nil {
		return nil, err
	}
	if err := db.Find(&groups).Error; err != nil {
		return nil, err
	}
	return NewResolver(book, prices, groups), nil
}

// Book is the official price book the resolver divides by.
func (r *Resolver) Book() *Book { return r.book }

// ChannelSpec is what the resolver needs to know about one channel.
type ChannelSpec struct {
	UpstreamID *uint // nil = 未关联上游
	HasToken   bool  // the channel's token exists
	TokenGroup string
	Models     []string           // the channel's models (for the median)
	Manual     *float64           // channel_ext.manual_discount; ≤ 0 counts as unset
	Overrides  map[string]float64 // channel_ext.model_discount
}

// ChannelRate is the resolved discount of one channel.
type ChannelRate struct {
	Discount   *float64 // channel-level discount; nil when Source == none
	Source     string   // manual / auto / group / none
	Reason     string   // why Discount is nil (Source == none only)
	GroupRatio *float64 // 令牌所在分组的上游分组倍率 (nil when unknown or ≤ 0)
	Median     *float64 // 渠道各模型 上游价 ÷ 原厂价 的中位数 (raw, nil when none derivable)
	Derivation Derivation

	res       *Resolver
	prices    map[string]UpstreamPrice
	manual    *float64
	overrides map[string]float64
	base      *float64 // deviation baseline
}

// Channel resolves the channel-level discount: manual → auto → group → none.
func (r *Resolver) Channel(spec ChannelSpec) *ChannelRate {
	cr := &ChannelRate{res: r, overrides: spec.Overrides}
	if spec.Manual != nil && *spec.Manual > 0 {
		v := *spec.Manual
		cr.manual = &v
	}
	if spec.UpstreamID != nil && spec.HasToken {
		ref := groupRef{*spec.UpstreamID, spec.TokenGroup}
		cr.prices = r.prices[ref]
		if g, ok := r.groups[ref]; ok && g > 0 {
			v := g
			cr.GroupRatio = &v
		}
	}
	cr.Derivation = Derive(spec.Models, r.book, cr.prices)
	cr.Median = cr.Derivation.Median
	cr.base = cr.GroupRatio
	if cr.base == nil {
		cr.base = cr.Median
	}
	for i := range cr.Derivation.Models {
		md := &cr.Derivation.Models[i]
		md.Deviation, md.Deviates = cr.deviation(md.Discount)
	}
	switch {
	case cr.manual != nil:
		cr.Discount, cr.Source = cr.manual, SourceManual
	case cr.Median != nil:
		v := RoundDiscount(*cr.Median)
		cr.Discount, cr.Source = &v, SourceAuto
	case cr.GroupRatio != nil:
		cr.Discount, cr.Source = cr.GroupRatio, SourceGroup
	default:
		cr.Source = SourceNone
		switch {
		case spec.UpstreamID == nil:
			cr.Reason = "未关联上游"
		case !spec.HasToken:
			cr.Reason = "未关联令牌"
		case spec.TokenGroup == "":
			cr.Reason = "令牌没有分组"
		default:
			cr.Reason = "上游没有这个分组的价格和倍率"
		}
	}
	return cr
}

func (cr *ChannelRate) deviation(own *float64) (*float64, bool) {
	if own == nil || cr.base == nil || *cr.base <= 0 {
		return nil, false
	}
	dev := (*own - *cr.base) / *cr.base
	return &dev, math.Abs(dev) > ModelDeviationThreshold
}

// ModelRate is the discount used to cost one model of a channel.
type ModelRate struct {
	Discount  *float64 // nil when Source == none
	Source    string   // override / manual / model / channel / none
	Official  string   // price-book name (after alias)
	Own       *float64 // the model's own 上游价 ÷ 原厂价
	Reason    string   // why Own is nil
	Deviation *float64 // Own relative to the baseline
	Deviates  bool     // |Deviation| > 1%
}

// Model resolves the discount for one model: override → manual → model → channel → none.
func (cr *ChannelRate) Model(model string) ModelRate {
	md := deriveOne(cr.res.book, model, cr.prices)
	mr := ModelRate{Official: md.Official, Own: md.Discount, Reason: md.Reason}
	mr.Deviation, mr.Deviates = cr.deviation(md.Discount)
	switch {
	case cr.overrides[model] > 0:
		v := cr.overrides[model]
		mr.Discount, mr.Source = &v, ModelOverride
	case cr.manual != nil:
		mr.Discount, mr.Source = cr.manual, ModelManual
	case md.Discount != nil:
		mr.Discount, mr.Source = md.Discount, ModelOwn
	case cr.Source == SourceAuto || cr.Source == SourceGroup:
		mr.Discount, mr.Source = cr.Discount, ModelChannel
	default:
		mr.Source = ModelNone
	}
	return mr
}

// Upstream returns the upstream price of a model on this channel's token group
// (the model name first, then its alias).
func (cr *ChannelRate) Upstream(model string) (UpstreamPrice, bool) {
	return lookupUpstream(cr.res.book, model, cr.prices)
}

// ModelDiscounts decodes channel_ext.model_discount (manual per-model overrides).
func ModelDiscounts(raw string) map[string]float64 {
	out := map[string]float64{}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &out)
	}
	return out
}
