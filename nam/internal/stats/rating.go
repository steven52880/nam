package stats

import (
	"context"
	"slices"
	"sort"
	"time"

	"github.com/steven52880/nam/internal/pricing"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

const (
	minRatedRequests = 20 // below this the quality tier is "insufficient"
	mixDays          = 7  // vendor token mix lookback
)

var tierNames = [3]string{TierCheap, TierMid, TierExpensive}

// ratings carries the per-channel std_cost_price and price tier.
type ratings struct {
	std  map[int]*float64
	tier map[int]string
}

// stdEntry is one channel with a standard-load cost price.
type stdEntry struct {
	id     int
	vendor string
	price  float64
}

// qualityTier grades a channel by its Wilson lower bound.
func qualityTier(requests int64, wilson *float64) string {
	if requests < minRatedRequests || wilson == nil {
		return QualityInsufficient
	}
	switch {
	case *wilson >= 0.99:
		return QualityGood
	case *wilson >= 0.95:
		return QualityMid
	default:
		return QualityBad
	}
}

// priceTiers ranks channels inside each vendor (ascending price, id breaks ties):
// tier = floor(3i/n). Vendors with fewer than 2 priced channels, and channels without
// a vendor, get no tier.
func priceTiers(entries []stdEntry) map[int]string {
	byVendor := map[string][]stdEntry{}
	for _, e := range entries {
		if e.vendor != "" {
			byVendor[e.vendor] = append(byVendor[e.vendor], e)
		}
	}
	out := map[int]string{}
	for _, g := range byVendor {
		n := len(g)
		if n < 2 {
			continue
		}
		sort.Slice(g, func(i, j int) bool {
			if g[i].price != g[j].price {
				return g[i].price < g[j].price
			}
			return g[i].id < g[j].id
		})
		for i, e := range g {
			out[e.id] = tierNames[3*i/n]
		}
	}
	return out
}

// vendorMix sums the last 7 days of tokens per vendor from stats_hourly.
func (s *Service) vendorMix(ctx context.Context) (map[string]pricing.Usage, error) {
	from := time.Now().Unix() - mixDays*86400
	from -= from % 3600
	rows, err := s.St.DB.WithContext(ctx).Model(&store.StatsHourly{}).
		Select("vendor, SUM(prompt_tokens), SUM(cache_read_tokens), SUM(cache_write_tokens), SUM(completion_tokens)").
		Where("hour >= ?", from).Group("vendor").Rows()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]pricing.Usage{}
	for rows.Next() {
		var vendor string
		var in, read, write, output int64
		if err := rows.Scan(&vendor, &in, &read, &write, &output); err != nil {
			return nil, err
		}
		if in+read+write+output == 0 {
			continue
		}
		out[vendor] = pricing.Usage{
			Input: float64(in), CacheRead: float64(read), CacheWrite5m: float64(write), Output: float64(output), Requests: 1,
		}
	}
	return out, rows.Err()
}

// stdPrice is the channel's cost as a multiple of the official price (unitless, already
// divided by the 美元参考汇率) under the vendor's token mix, on the
// channel's test model (or the cheapest priced one). nil when unmanaged or unpriced.
// A non-empty model overrides that choice: the price is taken on exactly that model.
func stdPrice(pr *Pricer, ch ports.Channel, mix map[string]pricing.Usage, model string) *float64 {
	cp, ok := pr.channels[ch.ID]
	if !ok {
		return nil
	}
	if model == "" {
		model = ch.TestModel
		if _, priced := pr.book.Resolve(model); !priced {
			model = pr.book.CheapestModel(ch.Models)
		}
	}
	p, ok := pr.book.Resolve(model)
	if !ok {
		return nil
	}
	u, ok := mix[cp.ext.Vendor]
	if !ok {
		u = pricing.StandardRequest
	}
	base := pricing.OfficialUSD(p, u, pricing.CacheRatio{})
	if base == 0 {
		return nil
	}
	disc := pricing.EffectiveDiscount(cp.ext.Discount, cp.disc, model)
	v := pricing.ChannelCNY(pricing.OfficialUSD(p, u, cp.cache), disc, cp.rate) / base / pr.usdRate
	return &v
}

// rate computes std_cost_price and price tiers for every listed channel. A non-empty
// model restricts the computation to channels that serve that model, so price tiers only
// compare channels among themselves that carry it.
func (s *Service) rate(ctx context.Context, pr *Pricer, chans []ports.Channel, model string) (ratings, error) {
	mix, err := s.vendorMix(ctx)
	if err != nil {
		return ratings{}, err
	}
	r := ratings{std: map[int]*float64{}}
	var entries []stdEntry
	for _, ch := range chans {
		if model != "" && !slices.Contains(ch.Models, model) {
			continue
		}
		v := stdPrice(pr, ch, mix, model)
		r.std[ch.ID] = v
		if v != nil {
			entries = append(entries, stdEntry{id: ch.ID, vendor: pr.channels[ch.ID].ext.Vendor, price: *v})
		}
	}
	r.tier = priceTiers(entries)
	return r, nil
}

func (r ratings) tierPtr(id int) *string {
	if t, ok := r.tier[id]; ok {
		return &t
	}
	return nil
}
