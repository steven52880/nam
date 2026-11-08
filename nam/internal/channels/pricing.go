package channels

import (
	"context"

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
		p, ok := book.Resolve(model)
		if !ok {
			continue
		}
		overrides := map[string]float64{}
		cache := pricing.CacheRatio{}
		if e, ok := st.ext[r.ID]; ok {
			overrides = pricing.ModelDiscounts(e.ModelDiscount)
			cache = pricing.ParseCacheRatio(e.CacheOverride)
		}
		disc := pricing.EffectiveDiscount(r.Discount, overrides, model)
		usd := pricing.OfficialUSD(p, pricing.StandardRequest, cache)
		cny := pricing.ChannelCNY(usd, disc, *r.CurrencyRate)
		r.UnitCost, r.UnitCostModel = &cny, model
	}
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
