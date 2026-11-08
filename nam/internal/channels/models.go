package channels

import (
	"context"

	"github.com/steven52880/nam/internal/pricing"
	"github.com/steven52880/nam/ports"
)

// ModelRow is one model of a channel in the lower detail table.
type ModelRow struct {
	Model          string         `json:"model"`
	Official       string         `json:"official"`         // 原厂价库里的名字（别名映射后）
	Price          *pricing.Price `json:"price"`            // 原厂价; nil = 缺价
	UpstreamInput  *float64       `json:"upstream_input"`   // 上游价（上游美元 / 百万 token，已含上游分组倍率）
	Derived        *float64       `json:"derived"`          // 由上游价推算的折扣
	Deviation      *float64       `json:"deviation"`        // 相对渠道中位数
	Deviates       bool           `json:"deviates"`         // 偏离 > 5%
	Reason         string         `json:"reason"`           // 无法推算的原因
	Override       *float64       `json:"override"`         // 模型级上游分组折扣覆盖
	Effective      float64        `json:"effective"`        // 实际使用的折扣
	CostInPerM     *float64       `json:"cost_in_per_m"`    // 成本（元 / 百万输入 token）
	CostOutPerM    *float64       `json:"cost_out_per_m"`   // 成本（元 / 百万输出 token）
	CostPerRequest *float64       `json:"cost_per_request"` // 按次计费模型的单次成本（元）
}

// ModelDetail is the lower table for one channel.
type ModelDetail struct {
	ChannelID      int        `json:"channel_id"`
	Name           string     `json:"name"`
	Vendor         string     `json:"vendor"`
	Discount       float64    `json:"discount"`
	DiscountSource string     `json:"discount_source"`
	DerivedMedian  *float64   `json:"derived_median"` // 当前上游价推算出的中位数（自动来源会在倍率同步后采用）
	CurrencyRate   *float64   `json:"currency_rate"`
	TestModel      string     `json:"test_model"`
	Models         []ModelRow `json:"models"`
}

// Models returns the per-model price/discount/cost breakdown of one channel
// (works for drafts too: values come from the overlaid row).
func (s *Service) Models(ctx context.Context, id int) (ModelDetail, error) {
	st, err := s.load(ctx)
	if err != nil {
		return ModelDetail{}, err
	}
	if _, live := st.liveMap[id]; !live {
		if _, draft := st.drafts[id]; !draft {
			return ModelDetail{}, ports.ErrNotFound
		}
	}
	r := st.row(id)
	book, err := pricing.LoadBook(ctx, s.St)
	if err != nil {
		return ModelDetail{}, err
	}
	out := ModelDetail{ChannelID: id, Name: r.Name, Vendor: r.Vendor, Discount: r.Discount, DiscountSource: r.DiscountSource,
		CurrencyRate: r.CurrencyRate, TestModel: r.TestModel, Models: make([]ModelRow, 0, len(r.Models))}

	up := map[string]pricing.UpstreamPrice{}
	if r.UpstreamID != nil && r.TokenID != nil {
		if t, ok := st.tokens[*r.TokenID]; ok {
			if up, err = pricing.UpstreamPrices(ctx, s.St, *r.UpstreamID, t.Group); err != nil {
				return ModelDetail{}, err
			}
		}
	}
	d := pricing.Derive(r.Models, book, up)
	out.DerivedMedian = d.Median
	overrides := map[string]float64{}
	if e, ok := st.ext[id]; ok {
		overrides = pricing.ModelDiscounts(e.ModelDiscount)
	}
	for i, m := range r.Models {
		md := d.Models[i]
		row := ModelRow{Model: m, Official: md.Official, Derived: md.Discount, Deviation: md.Deviation, Deviates: md.Deviates, Reason: md.Reason}
		if u, ok := up[m]; ok {
			row.UpstreamInput = u.Input
		}
		if v, ok := overrides[m]; ok {
			row.Override = &v
		}
		row.Effective = pricing.EffectiveDiscount(r.Discount, overrides, m)
		if p, ok := book.Resolve(m); ok {
			pp := p
			row.Price = &pp
			if r.CurrencyRate != nil {
				if p.IsPerRequest() {
					c := pricing.ChannelCNY(*p.PerRequest, row.Effective, *r.CurrencyRate)
					row.CostPerRequest = &c
				} else {
					row.CostInPerM, row.CostOutPerM = pricing.PerMillionCNY(p, row.Effective, *r.CurrencyRate)
				}
			}
		}
		out.Models = append(out.Models, row)
	}
	return out, nil
}
