package channels

import (
	"context"
	"slices"

	"github.com/steven52880/nam/internal/pricing"
)

// spec is what the resolver needs about a row: its upstream, token group, models, manual
// discount (draft overlaid) and per-model overrides.
func (st *state) spec(r Row) pricing.ChannelSpec {
	sp := pricing.ChannelSpec{UpstreamID: r.UpstreamID, Models: r.Models, Manual: r.manual}
	if r.TokenID != nil {
		if t, ok := st.tokens[*r.TokenID]; ok {
			sp.HasToken, sp.TokenGroup = true, t.Group
		}
	}
	if e, ok := st.ext[r.ID]; ok {
		sp.Overrides = pricing.ModelDiscounts(e.ModelDiscount)
	}
	return sp
}

// channelRate resolves a row's channel-level discount.
func (st *state) channelRate(rs *pricing.Resolver, r Row) *pricing.ChannelRate {
	return rs.Channel(st.spec(r))
}

// cacheOf is the channel's cache-ratio override (empty when it has no ext row).
func (st *state) cacheOf(id int) pricing.CacheRatio {
	if e, ok := st.ext[id]; ok {
		return pricing.ParseCacheRatio(e.CacheOverride)
	}
	return pricing.CacheRatio{}
}

// priceRows fills discount, cost columns and balance: 倍率 (manual → auto → group → none),
// 上游渠道售价, 单次成本 (standard request on the test model, or the cheapest priced model)
// and upstream balance. A null discount leaves every cost field null.
func (st *state) priceRows(rows []Row, rs *pricing.Resolver) {
	book := rs.Book()
	for i := range rows {
		r := &rows[i]
		if r.UpstreamID != nil {
			if u, ok := st.ups[*r.UpstreamID]; ok && u.BalanceUSD != nil {
				usd := *u.BalanceUSD
				cny := usd * u.CurrencyRate
				r.BalanceUSD, r.BalanceCNY = &usd, &cny
			}
		}
		cr := st.channelRate(rs, *r)
		r.Discount, r.DiscountSource, r.DiscountReason, r.GroupRatio = cr.Discount, cr.Source, cr.Reason, cr.GroupRatio
		if r.CurrencyRate == nil {
			continue
		}
		if cr.Discount != nil {
			cost := st.channelCost(*cr.Discount, *r.CurrencyRate)
			r.ChannelCost = &cost
		}
		if len(r.Models) == 0 {
			continue
		}
		model := r.TestModel
		if _, ok := book.Resolve(model); !ok {
			model = book.CheapestModel(r.Models)
		}
		mr := cr.Model(model)
		if mr.Discount == nil {
			continue
		}
		cny := unitCostCNY(book, model, *mr.Discount, *r.CurrencyRate, st.cacheOf(r.ID))
		if cny == nil {
			continue
		}
		r.UnitCost, r.UnitCostModel = cny, model
	}
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
	Source      string   `json:"discount_source"` // override / manual / model / channel / none
	ChannelCost *float64 `json:"channel_cost"`    // 原厂价的倍数，无单位（见 Row.ChannelCost）
	UnitCost    *float64 `json:"unit_cost"`
}

// ModelCosts computes every channel row's discount and cost on model. Rows that do not
// carry the model have HasModel=false and nil numbers. Row set equals List (incl. drafts).
func (s *Service) ModelCosts(ctx context.Context, model string) ([]ModelCost, error) {
	st, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	rs, err := s.resolver(ctx)
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
		mr := st.channelRate(rs, r).Model(model)
		mc.Discount, mc.Source = mr.Discount, mr.Source
		if mr.Discount != nil && r.CurrencyRate != nil {
			cc := st.channelCost(*mr.Discount, *r.CurrencyRate)
			mc.ChannelCost = &cc
			mc.UnitCost = unitCostCNY(rs.Book(), model, *mr.Discount, *r.CurrencyRate, st.cacheOf(r.ID))
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

// DeriveItem is one channel's derivation preview (POST /upstreams/:id/derive).
type DeriveItem struct {
	ChannelID  int                `json:"channel_id"`
	Name       string             `json:"name"`
	Group      string             `json:"group"` // the token's group on the upstream
	Discount   *float64           `json:"discount"`
	Source     string             `json:"discount_source"`
	Reason     string             `json:"discount_reason"`
	GroupRatio *float64           `json:"upstream_group_ratio"`
	Derivation pricing.Derivation `json:"derivation"`
}

// Derivations previews the discount of every channel that belongs to the upstream
// (managed channels that exist in NewAPI). It writes nothing.
func (s *Service) Derivations(ctx context.Context, upstreamID uint) ([]DeriveItem, error) {
	st, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	rs, err := s.resolver(ctx)
	if err != nil {
		return nil, err
	}
	out := []DeriveItem{}
	for _, r := range st.rows() {
		if _, live := st.liveMap[r.ID]; !live || r.UpstreamID == nil || *r.UpstreamID != upstreamID || !r.Managed {
			continue
		}
		cr := st.channelRate(rs, r)
		it := DeriveItem{ChannelID: r.ID, Name: r.Name, Discount: cr.Discount, Source: cr.Source, Reason: cr.Reason,
			GroupRatio: cr.GroupRatio, Derivation: cr.Derivation}
		if r.TokenID != nil {
			it.Group = st.tokens[*r.TokenID].Group
		}
		out = append(out, it)
	}
	return out, nil
}

// ModelUse is one channel serving a model (by name or by alias), with its cost for it
// (模型与价格页).
type ModelUse struct {
	ID          int
	Name        string
	Vendor      string
	Discount    *float64
	Source      string // override / manual / model / channel / none
	Override    *float64
	CostInPerM  *float64
	CostOutPerM *float64
	CostPerReq  *float64
}

// ModelUses lists every channel (drafts overlaid) whose models include `model` or an alias
// of the same official model, with the cost on the model it serves.
func (s *Service) ModelUses(ctx context.Context, model string) ([]ModelUse, error) {
	st, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	rs, err := s.resolver(ctx)
	if err != nil {
		return nil, err
	}
	book := rs.Book()
	price, priced := book.Resolve(model)
	want := book.Canonical(model)
	uses := []ModelUse{}
	for _, r := range st.rows() {
		served := ""
		for _, m := range r.Models {
			if m == model || book.Canonical(m) == want {
				served = m
				break
			}
		}
		if served == "" {
			continue
		}
		mr := st.channelRate(rs, r).Model(served)
		u := ModelUse{ID: r.ID, Name: r.Name, Vendor: r.Vendor, Discount: mr.Discount, Source: mr.Source}
		if v, ok := st.spec(r).Overrides[served]; ok {
			u.Override = &v
		}
		if priced && r.CurrencyRate != nil && mr.Discount != nil {
			if price.IsPerRequest() {
				v := pricing.ChannelCNY(*price.PerRequest, *mr.Discount, *r.CurrencyRate)
				u.CostPerReq = &v
			} else {
				u.CostInPerM, u.CostOutPerM = pricing.PerMillionCNY(price, *mr.Discount, *r.CurrencyRate)
			}
		}
		uses = append(uses, u)
	}
	return uses, nil
}
