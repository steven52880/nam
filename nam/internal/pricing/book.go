// Package pricing holds nam's price math: official price resolution, upstream discount
// derivation and cost formulas. It never writes NewAPI.
//
// Units: official prices are USD per 1M tokens (or USD per request). Upstream prices
// are in upstream-USD per 1M tokens. Costs are CNY.
package pricing

import (
	"context"

	"github.com/steven52880/nam/internal/store"
)

// Price is a resolved official price.
type Price struct {
	Model        string   `json:"model"` // name in the price book (after alias)
	Input        *float64 `json:"input"`
	Output       *float64 `json:"output"`
	CacheRead    *float64 `json:"cache_read"`
	CacheWrite5m *float64 `json:"cache_write_5m"`
	CacheWrite1h *float64 `json:"cache_write_1h"`
	PerRequest   *float64 `json:"per_request"`
	Source       string   `json:"source"` // book source, or "override"
}

// IsPerRequest reports per-request billing.
func (p Price) IsPerRequest() bool { return p.PerRequest != nil && p.Input == nil }

// Valid reports whether the price can be used for cost math.
func (p Price) Valid() bool {
	if p.IsPerRequest() {
		return *p.PerRequest > 0
	}
	return p.Input != nil && *p.Input > 0
}

// Book resolves official prices: manual override > alias → book > book.
type Book struct {
	book      map[string]store.OfficialPrice
	overrides map[string]store.PriceOverride
	aliases   map[string]string
}

// LoadBook reads the price tables.
func LoadBook(ctx context.Context, st *store.Store) (*Book, error) {
	var ps []store.OfficialPrice
	var os []store.PriceOverride
	var as []store.ModelAlias
	db := st.DB.WithContext(ctx)
	if err := db.Find(&ps).Error; err != nil {
		return nil, err
	}
	if err := db.Find(&os).Error; err != nil {
		return nil, err
	}
	if err := db.Find(&as).Error; err != nil {
		return nil, err
	}
	b := &Book{book: map[string]store.OfficialPrice{}, overrides: map[string]store.PriceOverride{}, aliases: map[string]string{}}
	for _, p := range ps {
		b.book[p.Model] = p
	}
	for _, o := range os {
		b.overrides[o.Model] = o
	}
	for _, a := range as {
		b.aliases[a.Alias] = a.Official
	}
	return b, nil
}

// Canonical maps a (possibly upstream) model name to the price book name.
func (b *Book) Canonical(model string) string {
	if o, ok := b.aliases[model]; ok {
		return o
	}
	return model
}

// Resolve returns the official price for a model. ok=false means "缺价".
func (b *Book) Resolve(model string) (Price, bool) {
	name := b.Canonical(model)
	for _, n := range []string{model, name} {
		if o, ok := b.overrides[n]; ok {
			p := Price{Model: n, Source: "override", CacheRead: o.CacheRead, CacheWrite5m: o.CacheWrite5m, CacheWrite1h: o.CacheWrite1h}
			if o.PerRequest {
				p.PerRequest = o.PerRequestUSD
			} else {
				p.Input, p.Output = o.Input, o.Output
			}
			return p, p.Valid()
		}
	}
	bp, ok := b.book[name]
	if !ok {
		return Price{Model: name}, false
	}
	p := Price{Model: name, Input: bp.Input, Output: bp.Output, CacheRead: bp.CacheRead, CacheWrite5m: bp.CacheWrite5m,
		CacheWrite1h: bp.CacheWrite1h, PerRequest: bp.PerRequest, Source: bp.Source}
	return p, p.Valid()
}

// Has reports whether the book (or an override) knows the model.
func (b *Book) Has(model string) bool {
	_, ok := b.Resolve(model)
	return ok
}

// CheapestModel returns the model with the lowest input (or per-request) price,
// falling back to the first model when none is priced. Used for channel test_model.
func (b *Book) CheapestModel(models []string) string {
	best, bestPrice := "", 0.0
	for _, m := range models {
		p, ok := b.Resolve(m)
		if !ok {
			continue
		}
		v := 0.0
		if p.IsPerRequest() {
			v = *p.PerRequest * 1e6 // compare per-request as if 1M tokens: they are rarely the cheapest test
		} else {
			v = *p.Input
			if p.Output != nil {
				v += *p.Output
			}
		}
		if best == "" || v < bestPrice {
			best, bestPrice = m, v
		}
	}
	if best == "" && len(models) > 0 {
		return models[0]
	}
	return best
}
