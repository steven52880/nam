package channels

import (
	"context"
	"slices"

	"github.com/steven52880/nam/internal/pricing"
)

// PricingInputs lists managed live channels with their upstream token group, for
// pricing.RefreshAuto (自动推算上游分组折扣).
func (s *Service) PricingInputs(ctx context.Context) ([]pricing.ChannelInput, error) {
	st, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	var out []pricing.ChannelInput
	for id, e := range st.ext {
		live, ok := st.liveMap[id]
		if !ok || e.UpstreamID == nil {
			continue
		}
		group := ""
		if e.TokenID != nil {
			if t, ok := st.tokens[*e.TokenID]; ok {
				group = t.Group
			}
		}
		out = append(out, pricing.ChannelInput{ChannelID: id, UpstreamID: *e.UpstreamID, Group: group, Models: live.Models})
	}
	return out, nil
}

// priceRows fills cost columns: 单次成本 (standard request on the test model, or the
// cheapest priced model) and upstream balance. Missing prices leave the fields nil.
func priceRows(rows []Row, st *state, book *pricing.Book) {
	for i := range rows {
		r := &rows[i]
		if r.UpstreamID != nil {
			if u, ok := st.ups[*r.UpstreamID]; ok && u.BalanceUSD != nil {
				usd := *u.BalanceUSD
				cny := usd * u.CurrencyRate
				r.BalanceUSD, r.BalanceCNY = &usd, &cny
			}
		}
		if r.CurrencyRate == nil || len(r.Models) == 0 {
			continue
		}
		model := r.TestModel
		if _, ok := book.Resolve(model); !ok {
			model = book.CheapestModel(r.Models)
		}
		overrides, cache := costInputs(st, r.ID)
		disc := pricing.EffectiveDiscount(r.Discount, overrides, model)
		cny := unitCostCNY(book, model, disc, *r.CurrencyRate, cache)
		if cny == nil {
			continue
		}
		r.UnitCost, r.UnitCostModel = cny, model
	}
}

// costInputs returns a channel's per-model discount overrides and cache-ratio override
// (empty when the channel has no ext row).
func costInputs(st *state, id int) (overrides map[string]float64, cache pricing.CacheRatio) {
	overrides = map[string]float64{}
	if e, ok := st.ext[id]; ok {
		overrides = pricing.ModelDiscounts(e.ModelDiscount)
		cache = pricing.ParseCacheRatio(e.CacheOverride)
	}
	return overrides, cache
}

// unitCostCNY is the cost of one standard request on model, or nil when the model has no
// price in the book.
func unitCostCNY(book *pricing.Book, model string, disc, rate float64, cache pricing.CacheRatio) *float64 {
	p, ok := book.Resolve(model)
	if !ok {
		return nil
	}
	usd := pricing.OfficialUSD(p, pricing.StandardRequest, cache)
	cny := pricing.ChannelCNY(usd, disc, rate)
	return &cny
}

// ModelCost is one channel's cost for a single model (GET /channels/costs).
type ModelCost struct {
	ChannelID   int      `json:"channel_id"`
	HasModel    bool     `json:"has_model"`
	Discount    *float64 `json:"discount"`
	ChannelCost *float64 `json:"channel_cost"` // 原厂价的倍数，无单位（见 Row.ChannelCost）
	UnitCost    *float64 `json:"unit_cost"`
}

// ModelCosts computes every channel row's discount and cost on model. Rows that do not
// carry the model have HasModel=false and nil numbers. Row set equals List (incl. drafts).
func (s *Service) ModelCosts(ctx context.Context, model string) ([]ModelCost, error) {
	st, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	book, err := pricing.LoadBook(ctx, s.St)
	if err != nil {
		return nil, err
	}
	rows := st.rows()
	out := make([]ModelCost, 0, len(rows))
	for _, r := range rows {
		mc := ModelCost{ChannelID: r.ID}
		if !slices.Contains(r.Models, model) {
			out = append(out, mc)
			continue
		}
		mc.HasModel = true
		overrides, cache := costInputs(st, r.ID)
		disc := pricing.EffectiveDiscount(r.Discount, overrides, model)
		mc.Discount = &disc
		if r.CurrencyRate != nil {
			cc := st.channelCost(disc, *r.CurrencyRate)
			mc.ChannelCost = &cc
			mc.UnitCost = unitCostCNY(book, model, disc, *r.CurrencyRate, cache)
		}
		out = append(out, mc)
	}
	return out, nil
}

// testModelFor picks the default test model for a batch-created channel: the cheapest
// model by official price ("选该渠道最便宜的模型"), falling back to the first model.
func testModelFor(book *pricing.Book, models []string) string {
	if book == nil {
		if len(models) > 0 {
			return models[0]
		}
		return ""
	}
	return book.CheapestModel(models)
}
