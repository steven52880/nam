package channels

import (
	"context"

	"github.com/steven52880/nam/internal/pricing"
	"github.com/steven52880/nam/ports"
)

// ModelRow is one model of a channel in the lower detail table.
type ModelRow struct {
	Model           string         `json:"model"`
	Official        string         `json:"official"`         // 原厂价库里的名字（别名映射后）
	Price           *pricing.Price `json:"price"`            // 原厂价; nil = 缺价
	UpstreamInput   *float64       `json:"upstream_input"`   // 上游价（上游美元 / 百万 token，已含上游分组倍率）
	Derived         *float64       `json:"derived"`          // 这个模型自己的 上游价 ÷ 原厂价
	Deviation       *float64       `json:"deviation"`        // 相对基准（上游分组倍率，没有则渠道中位数）
	Deviates        bool           `json:"deviates"`         // 偏离 > 1%：上游给这个模型的标价和原厂价不一致
	Reason          string         `json:"reason"`           // 无法推算的原因
	Override        *float64       `json:"override"`         // 模型级倍率覆盖
	Effective       *float64       `json:"effective"`        // 实际使用的倍率；null = 算不出
	EffectiveSource string         `json:"effective_source"` // override / manual / model / channel / none
	CostInPerM      *float64       `json:"cost_in_per_m"`    // 成本（元 / 百万输入 token）
	CostOutPerM     *float64       `json:"cost_out_per_m"`   // 成本（元 / 百万输出 token）
	CostPerRequest  *float64       `json:"cost_per_request"` // 按次计费模型的单次成本（元）
}

// ModelDetail is the lower table for one channel.
type ModelDetail struct {
	ChannelID      int        `json:"channel_id"`
	Name           string     `json:"name"`
	Vendor         string     `json:"vendor"`
	Discount       *float64   `json:"discount"`
	DiscountSource string     `json:"discount_source"`
	DiscountReason string     `json:"discount_reason"`
	GroupRatio     *float64   `json:"upstream_group_ratio"`
	DerivedMedian  *float64   `json:"derived_median"` // 渠道各模型 上游价 ÷ 原厂价 的中位数
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
	rs, err := s.resolver(ctx)
	if err != nil {
		return ModelDetail{}, err
	}
	book := rs.Book()
	cr := st.channelRate(rs, r)
	out := ModelDetail{ChannelID: id, Name: r.Name, Vendor: r.Vendor, Discount: cr.Discount, DiscountSource: cr.Source,
		DiscountReason: cr.Reason, GroupRatio: cr.GroupRatio, DerivedMedian: cr.Median,
		CurrencyRate: r.CurrencyRate, TestModel: r.TestModel, Models: make([]ModelRow, 0, len(r.Models))}
	overrides := st.spec(r).Overrides
	for _, m := range r.Models {
		mr := cr.Model(m)
		row := ModelRow{Model: m, Official: mr.Official, Derived: mr.Own, Deviation: mr.Deviation, Deviates: mr.Deviates,
			Reason: mr.Reason, Effective: mr.Discount, EffectiveSource: mr.Source}
		if u, ok := cr.Upstream(m); ok {
			row.UpstreamInput = u.Input
		}
		if v, ok := overrides[m]; ok {
			row.Override = &v
		}
		if p, ok := book.Resolve(m); ok {
			pp := p
			row.Price = &pp
			if r.CurrencyRate != nil && mr.Discount != nil {
				if p.IsPerRequest() {
					c := pricing.ChannelCNY(*p.PerRequest, *mr.Discount, *r.CurrencyRate)
					row.CostPerRequest = &c
				} else {
					row.CostInPerM, row.CostOutPerM = pricing.PerMillionCNY(p, *mr.Discount, *r.CurrencyRate)
				}
			}
		}
		out.Models = append(out.Models, row)
	}
	return out, nil
}
