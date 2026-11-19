package stats

import (
	"context"
	"fmt"

	"github.com/steven52880/nam/internal/pricing"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/internal/vendors"
	"github.com/steven52880/nam/ports"
)

// defaultQuotaPerUnit is NewAPI's default QuotaPerUnit (quota per 1 USD).
const defaultQuotaPerUnit = 500000

// channelPrice is the pricing input of one managed channel.
type channelPrice struct {
	rate  float64            // upstream currency rate
	cache pricing.CacheRatio // per-channel cache price override
	disc  *pricing.ChannelRate
}

// Pricer prices samples from nam.db only (built once per pull; never calls NewAPI per log).
type Pricer struct {
	book         *pricing.Book
	channels     map[int]channelPrice // managed channels with an upstream
	vendorByGrp  map[string]string
	vendorOrder  []vendors.VendorGroup // vendor order, for the vendor a channel's groups imply
	chanVendor   map[int]string        // channel id → vendor implied by its current groups
	quotaPerUnit float64
	siteRate     float64
	usdRate      float64 // 美元参考汇率 (元 / 1 美元); divides every cost price
}

// NewPricer loads the price book, channel_ext, cached upstream prices and group ratios,
// upstreams, tokens and vendors, plus the NewAPI channel list (models and groups). A
// channel's discount is resolved per model by pricing.Resolver; a model without a discount
// is not priced (revenue still counts).
func NewPricer(ctx context.Context, st *store.Store, backend ports.Backend, chs ports.ChannelStore) (*Pricer, error) {
	book, err := pricing.LoadBook(ctx, st)
	if err != nil {
		return nil, fmt.Errorf("加载价格库：%w", err)
	}
	rs, err := pricing.LoadResolver(ctx, st, book)
	if err != nil {
		return nil, err
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
	var toks []store.UpstreamToken
	if err := db.Select("id", "upstream_id", "`group`").Find(&toks).Error; err != nil {
		return nil, err
	}
	var vs []store.Vendor
	if err := db.Select("name", "group_name").Order("`order`, id").Find(&vs).Error; err != nil {
		return nil, err
	}
	var live []ports.Channel
	if chs != nil {
		if live, err = chs.List(ctx); err != nil {
			return nil, fmt.Errorf("读取渠道：%w", err)
		}
	}
	rates := make(map[uint]float64, len(ups))
	for _, u := range ups {
		rates[u.ID] = u.CurrencyRate
	}
	tokGroup := make(map[uint]string, len(toks))
	for _, t := range toks {
		tokGroup[t.ID] = t.Group
	}
	liveByID := make(map[int]ports.Channel, len(live))
	for _, c := range live {
		liveByID[c.ID] = c
	}
	p := &Pricer{
		book: book, channels: map[int]channelPrice{}, chanVendor: map[int]string{},
		vendorByGrp: map[string]string{}, quotaPerUnit: defaultQuotaPerUnit,
		siteRate: st.GetFloat(ctx, store.KeySiteRate, 7), usdRate: st.USDRate(ctx),
	}
	for _, v := range vs {
		p.vendorByGrp[v.GroupName] = v.Name
		p.vendorOrder = append(p.vendorOrder, vendors.VendorGroup{Name: v.Name, Group: v.GroupName})
	}
	for _, c := range live {
		p.chanVendor[c.ID] = vendors.VendorOfGroups(p.vendorOrder, c.Group)
	}
	for _, e := range exts {
		if e.UpstreamID == nil {
			continue
		}
		rate, ok := rates[*e.UpstreamID]
		if !ok {
			continue
		}
		spec := pricing.ChannelSpec{UpstreamID: e.UpstreamID, Models: liveByID[e.ChannelID].Models,
			Manual: e.ManualDiscount, Overrides: pricing.ModelDiscounts(e.ModelDiscount)}
		if e.TokenID != nil {
			if g, ok := tokGroup[*e.TokenID]; ok {
				spec.HasToken, spec.TokenGroup = true, g
			}
		}
		p.channels[e.ChannelID] = channelPrice{rate: rate, cache: pricing.ParseCacheRatio(e.CacheOverride), disc: rs.Channel(spec)}
	}
	if backend != nil {
		// Backend down only costs the QuotaPerUnit lookup; the default matches NewAPI's.
		if info, err := backend.Info(ctx); err == nil && info.QuotaPerUnit > 0 {
			p.quotaPerUnit = info.QuotaPerUnit
		}
	}
	return p, nil
}

// Price computes revenue (always) and cost (when the model, channel and discount are known).
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
	mr := ch.disc.Model(s.Model)
	if mr.Discount == nil {
		return out // 倍率算不出：这个请求不计成本，收入照算
	}
	u := pricing.Usage{
		Input: float64(s.Input), CacheRead: float64(s.Read), CacheWrite5m: float64(s.W5), CacheWrite1h: float64(s.W1),
		Output: float64(s.Output), Requests: 1,
	}
	noCache := pricing.Usage{Input: float64(s.Input + s.Read + s.W5 + s.W1), Output: float64(s.Output), Requests: 1}
	out.OK = true
	out.Official = pricing.OfficialUSD(op, u, pricing.CacheRatio{})
	out.OfficialNoCache = pricing.OfficialUSD(op, noCache, pricing.CacheRatio{})
	out.Cost = pricing.ChannelCNY(pricing.OfficialUSD(op, u, ch.cache), *mr.Discount, ch.rate)
	return out
}

// VendorOf assigns a vendor at ingest: the vendor owning the log's group, else the vendor
// implied by the channel's current groups, else "".
func (p *Pricer) VendorOf(group string, channelID int) string {
	if v, ok := p.vendorByGrp[group]; ok && group != "" {
		return v
	}
	return p.chanVendor[channelID]
}

// vendorOfChannel is the vendor implied by a channel's groups.
func (p *Pricer) vendorOfChannel(ch ports.Channel) string {
	return vendors.VendorOfGroups(p.vendorOrder, ch.Group)
}
