package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
)

// rcPriceFields holds the four per-million-token prices; every value may be a string or a number.
type rcPriceFields struct {
	Input      json.RawMessage `json:"input_price"`
	Output     json.RawMessage `json:"output_price"`
	CacheRead  json.RawMessage `json:"cache_read_input_price"`
	CacheWrite json.RawMessage `json:"cache_creation_input_price"`
}

func (f rcPriceFields) any() bool {
	return rcNum(f.Input) != nil || rcNum(f.Output) != nil ||
		rcNum(f.CacheRead) != nil || rcNum(f.CacheWrite) != nil
}

// rcEffectiveConfig is effective_price_config: flat fields, or only tiers[], or {}.
type rcEffectiveConfig struct {
	rcPriceFields
	RequestPrice json.RawMessage `json:"request_price"`
	Tiers        []rcPriceFields `json:"tiers"`
}

type rcModel struct {
	Name         string            `json:"name"`
	ModelID      string            `json:"model_id"`
	InputPrice   json.RawMessage   `json:"input_price"`
	OutputPrice  json.RawMessage   `json:"output_price"`
	CacheRead    json.RawMessage   `json:"cache_read_input_price"`
	CacheWrite   json.RawMessage   `json:"cache_creation_input_price"`
	RequestPrice json.RawMessage   `json:"request_price"`
	Effective    rcEffectiveConfig `json:"effective_price_config"`
	TotalMult    json.RawMessage   `json:"effective_total_multiplier"`
}

func (m rcModel) id() string {
	// effective carries model_id (+ name); the public sample only has name, which is the id.
	if m.ModelID != "" {
		return m.ModelID
	}
	return m.Name
}

type rcModelGroup struct {
	Name   string    `json:"name"`
	Prefix string    `json:"prefix"`
	Models []rcModel `json:"models"`
}

func scale(v *float64, mult float64) *float64 {
	if v == nil {
		return nil
	}
	r := *v * mult
	return &r
}

// price turns one model entry into a ModelPrice. effective_price_config already contains the
// channel/model multipliers; only the raw fallback is multiplied by effective_total_multiplier.
func (m rcModel) price(group string) ModelPrice {
	mp := ModelPrice{Model: m.id(), Group: group}
	fields := m.Effective.rcPriceFields
	if !fields.any() && len(m.Effective.Tiers) > 0 {
		fields = m.Effective.Tiers[0]
	}
	if fields.any() {
		mp.Input, mp.Output = rcNum(fields.Input), rcNum(fields.Output)
		mp.CacheRead, mp.CacheWrite = rcNum(fields.CacheRead), rcNum(fields.CacheWrite)
	} else {
		mult := 1.0
		if t := rcNum(m.TotalMult); t != nil {
			mult = *t
		}
		mp.Input, mp.Output = scale(rcNum(m.InputPrice), mult), scale(rcNum(m.OutputPrice), mult)
		mp.CacheRead, mp.CacheWrite = scale(rcNum(m.CacheRead), mult), scale(rcNum(m.CacheWrite), mult)
	}
	if req := rcNum(m.Effective.RequestPrice); req != nil {
		mp.PerRequest = req
	} else if req := rcNum(m.RequestPrice); req != nil {
		mult := 1.0
		if t := rcNum(m.TotalMult); t != nil {
			mult = *t
		}
		mp.PerRequest = scale(req, mult)
	}
	return mp
}

func (rightCodeAdapter) Pricing(ctx context.Context, s Site) (Pricing, error) {
	ups, err := rcUpstreams(ctx, s)
	if err != nil {
		return Pricing{}, err
	}
	groups := make([]GroupRatio, 0, len(ups))
	for _, u := range ups {
		r := rcNum(u.Rate)
		if r == nil {
			continue
		}
		groups = append(groups, GroupRatio{Name: u.Prefix, Ratio: *r, Desc: u.Name})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })

	c, err := rcOpen(s)
	if err != nil {
		return Pricing{}, err
	}
	var resp struct {
		Upstreams []rcModelGroup `json:"upstreams"`
	}
	if err := c.do(ctx, http.MethodGet, "/models/effective", nil, &resp); err != nil {
		if errors.Is(err, ErrAuth) {
			return Pricing{}, fmt.Errorf("models/effective: %w", err)
		}
		resp.Upstreams = nil
		if perr := getJSON(ctx, s.client(), joinURL(s.BaseURL, "/models/public"), nil, &resp); perr != nil {
			return Pricing{}, fmt.Errorf("models/public: %w", perr)
		}
	}
	out := Pricing{Groups: groups, Models: []ModelPrice{}}
	for _, g := range resp.Upstreams {
		for _, m := range g.Models {
			if m.id() == "" {
				continue
			}
			out.Models = append(out.Models, m.price(g.Prefix))
		}
	}
	return out, nil
}
