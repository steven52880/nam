package upstream

import (
	"context"
	"fmt"
	"net/http"
	"sort"
)

// OneHub pricing (github.com/MartialBE/one-hub, main, fetched 2026-11-08).
//
// Sources:
//   - controller/group.go GetUserGroupRatio: GET /api/user_group_map → data = {symbol: UserGroup};
//     only public groups plus the caller's own group (own group needs the session cookie,
//     TrySetUserBySession does not read access tokens). model/user_group.go: ratio float64.
//   - relay/model.go AvailableModel: GET /api/available_model → data = {model: {groups, owned_by,
//     price}}; price is model.Price {type "tokens"|"times", input, output, extra_ratios}.
//   - model/price.go: DollarRate = 0.002 ($ per 1K tokens at ratio 1), defaults gpt-4 {15, 30}
//     (= $30 / $60 per 1M, so input/output are absolute ratios, not completion multipliers),
//     default extra ratios cached_read_tokens 0.1 and cached_write_tokens 1.25.
//   - relay/relay_util/quota.go: inputRatio = price.GetInput()*groupRatio, outputRatio likewise;
//     "times" prices cost int(1000 * inputRatio) quota per request.
//   - common/config/constants.go: QuotaPerUnit = 500000 quota per USD (read from /api/status).
//
// Conversion (q = quota_per_unit): a token costs `ratio` quota, so
//
//	USD per 1M tokens = ratio * group_ratio * 1e6 / q    (= ratio * group * 2 at q = 500000)
//	USD per request   = input * group_ratio * 1000 / q   (= input * group * 0.002 at q = 500000)
//
// UNVERIFIED: that every relay path multiplies token counts by these ratios exactly
// (checked against the default price table and the quota.go constants only).
// UNVERIFIED: cache extra ratios are multipliers on the input price.

const (
	ohExtraCacheRead  = "cached_read_tokens"
	ohExtraCacheWrite = "cached_write_tokens"
)

var ohDefaultExtra = map[string]float64{ohExtraCacheRead: 0.1, ohExtraCacheWrite: 1.25}

type ohGroup struct {
	Symbol string  `json:"symbol"`
	Name   string  `json:"name"`
	Ratio  float64 `json:"ratio"`
}

type ohPrice struct {
	Type        string             `json:"type"`
	Input       float64            `json:"input"`
	Output      float64            `json:"output"`
	ExtraRatios map[string]float64 `json:"extra_ratios"`
}

type ohAvailable struct {
	Groups []string `json:"groups"`
	Price  *ohPrice `json:"price"`
}

func (p ohPrice) extra(key string) float64 {
	if r, ok := p.ExtraRatios[key]; ok {
		return r
	}
	return ohDefaultExtra[key]
}

func ohModelPrice(model, group string, p ohPrice, gr, qpu float64) ModelPrice {
	mp := ModelPrice{Model: model, Group: group}
	if p.Type == "times" {
		mp.PerRequest = f64(p.Input * gr * 1000 / qpu)
		return mp
	}
	perM := 1e6 / qpu
	in := p.Input * gr * perM
	mp.ModelRatio = f64(p.Input)
	mp.Input = f64(in)
	mp.Output = f64(p.Output * gr * perM)
	mp.CacheRead = f64(in * p.extra(ohExtraCacheRead))
	mp.CacheWrite = f64(in * p.extra(ohExtraCacheWrite))
	return mp
}

func (oneHubAdapter) Pricing(ctx context.Context, s Site) (Pricing, error) {
	qpu, err := quotaPerUnit(ctx, s)
	if err != nil {
		return Pricing{}, err
	}
	// Both endpoints are public; a session only adds the user's private group.
	c, serr := ohSession(ctx, s)
	if serr != nil {
		c = &naSession{hc: s.client(), base: s.BaseURL, hdr: map[string]string{}}
	}
	var rawGroups map[string]ohGroup
	if err := callEnvelope(ctx, c.hc, http.MethodGet, joinURL(s.BaseURL, "/api/user_group_map"), c.hdr, nil, &rawGroups); err != nil {
		return Pricing{}, fmt.Errorf("user_group_map: %w", err)
	}
	var avail map[string]ohAvailable
	if err := callEnvelope(ctx, c.hc, http.MethodGet, joinURL(s.BaseURL, "/api/available_model"), c.hdr, nil, &avail); err != nil {
		return Pricing{}, fmt.Errorf("available_model: %w", err)
	}

	ratios := make(map[string]float64, len(rawGroups))
	groups := make([]GroupRatio, 0, len(rawGroups))
	for sym, g := range rawGroups {
		if g.Symbol != "" {
			sym = g.Symbol
		}
		ratios[sym] = g.Ratio
		groups = append(groups, GroupRatio{Name: sym, Ratio: g.Ratio, Desc: g.Name})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })

	names := make([]string, 0, len(avail))
	for m := range avail {
		names = append(names, m)
	}
	sort.Strings(names)

	out := Pricing{Groups: groups, Models: []ModelPrice{}}
	for _, m := range names {
		a := avail[m]
		if a.Price == nil {
			continue
		}
		gs := append([]string(nil), a.Groups...)
		sort.Strings(gs)
		for _, g := range gs {
			if gr, ok := ratios[g]; ok {
				out.Models = append(out.Models, ohModelPrice(m, g, *a.Price, gr, qpu))
			}
		}
	}
	return out, nil
}
