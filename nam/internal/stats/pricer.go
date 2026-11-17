package stats

import (
	"context"
	"fmt"

	"github.com/steven52880/nam/internal/pricing"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

// defaultQuotaPerUnit is NewAPI's default QuotaPerUnit (quota per 1 USD).
const defaultQuotaPerUnit = 500000

// channelPrice is the pricing input of one managed channel.
type channelPrice struct {
	ext   store.ChannelExt
	rate  float64 // upstream currency rate
	cache pricing.CacheRatio
	disc  map[string]float64 // per-model discount overrides
}

// Pricer prices samples from nam.db only (built once per pull; never calls NewAPI per log).
type Pricer struct {
	book         *pricing.Book
	channels     map[int]channelPrice     // managed channels with an upstream
	exts         map[int]store.ChannelExt // every channel_ext row (vendor fallback)
	vendorByGrp  map[string]string
	quotaPerUnit float64
	siteRate     float64
}

// NewPricer loads the price book, channel_ext, upstreams and vendors.
func NewPricer(ctx context.Context, st *store.Store, backend ports.Backend) (*Pricer, error) {
	book, err := pricing.LoadBook(ctx, st)
	if err != nil {
		return nil, fmt.Errorf("加载价格库：%w", err)
	}
	db := st.DB.WithContext(ctx)
	var exts []store.ChannelExt
	if err := db.Find(&exts).Error; err != nil {
		return nil, err
	}
	var ups []store.Upstream
	if err := db.Select("id", "currency_rate").Find(&ups).Error; err != nil {
		return nil, err
	}
	var vs []store.Vendor
	if err := db.Select("name", "group_name").Find(&vs).Error; err != nil {
		return nil, err
	}
	rates := make(map[uint]float64, len(ups))
	for _, u := range ups {
		rates[u.ID] = u.CurrencyRate
	}
	p := &Pricer{
		book: book, channels: map[int]channelPrice{}, exts: map[int]store.ChannelExt{},
		vendorByGrp: map[string]string{}, quotaPerUnit: defaultQuotaPerUnit,
		siteRate: st.GetFloat(ctx, store.KeySiteRate, 7),
	}
	for _, e := range exts {
		p.exts[e.ChannelID] = e
		if e.UpstreamID == nil {
			continue
		}
		rate, ok := rates[*e.UpstreamID]
		if !ok {
			continue
		}
		p.channels[e.ChannelID] = channelPrice{
			ext: e, rate: rate, cache: pricing.ParseCacheRatio(e.CacheOverride), disc: pricing.ModelDiscounts(e.ModelDiscount),
		}
	}
	for _, v := range vs {
		p.vendorByGrp[v.GroupName] = v.Name
	}
	if backend != nil {
		// Backend down only costs the QuotaPerUnit lookup; the default matches NewAPI's.
		if info, err := backend.Info(ctx); err == nil && info.QuotaPerUnit > 0 {
			p.quotaPerUnit = info.QuotaPerUnit
		}
	}
	return p, nil
}

// Price computes revenue (always) and cost (when the model and channel are priced).
// Failures are not priced.
func (p *Pricer) Price(s Sample) Priced {
	if !s.OK {
		return Priced{}
	}
	out := Priced{Revenue: float64(s.Quota) / p.quotaPerUnit * p.siteRate}
	ch, ok := p.channels[s.ChannelID]
	if !ok {
		return out
	}
	op, ok := p.book.Resolve(s.Model)
	if !ok {
		return out
	}
	u := pricing.Usage{
		Input: float64(s.Input), CacheRead: float64(s.Read), CacheWrite5m: float64(s.W5), CacheWrite1h: float64(s.W1),
		Output: float64(s.Output), Requests: 1,
	}
	noCache := pricing.Usage{Input: float64(s.Input + s.Read + s.W5 + s.W1), Output: float64(s.Output), Requests: 1}
	disc := pricing.EffectiveDiscount(ch.ext.Discount, ch.disc, s.Model)
	out.OK = true
	out.Official = pricing.OfficialUSD(op, u, pricing.CacheRatio{})
	out.OfficialNoCache = pricing.OfficialUSD(op, noCache, pricing.CacheRatio{})
	out.Cost = pricing.ChannelCNY(pricing.OfficialUSD(op, u, ch.cache), disc, ch.rate)
	return out
}

// VendorOf assigns a vendor at ingest: the vendor owning the log's group, else the
// channel's channel_ext.vendor, else "".
func (p *Pricer) VendorOf(group string, channelID int) string {
	if v, ok := p.vendorByGrp[group]; ok && group != "" {
		return v
	}
	return p.exts[channelID].Vendor
}
