package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// usdPerMillionAtRatio1: NewAPI ratio 1 == $0.002 / 1K tokens == $2 / 1M tokens
// (setting/ratio_setting: "1 === $0.002 / 1K tokens", USD = 500 quota per $1).
const usdPerMillionAtRatio1 = 2.0

func errIsAuth(err error) bool { return errors.Is(err, ErrAuth) }

type pricingItem struct {
	ModelName        string   `json:"model_name"`
	QuotaType        int      `json:"quota_type"`
	ModelRatio       float64  `json:"model_ratio"`
	ModelPrice       float64  `json:"model_price"`
	CompletionRatio  float64  `json:"completion_ratio"`
	CacheRatio       *float64 `json:"cache_ratio"`
	CreateCacheRatio *float64 `json:"create_cache_ratio"`
	EnableGroups     []string `json:"enable_groups"`
}

func (p pricingItem) inGroup(g string) bool {
	if len(p.EnableGroups) == 0 {
		return true
	}
	for _, eg := range p.EnableGroups {
		if eg == g || eg == "all" {
			return true
		}
	}
	return false
}

func f64(v float64) *float64 { return &v }

// modelPriceFor computes one (model, group) price in upstream USD.
func modelPriceFor(p pricingItem, g GroupRatio) ModelPrice {
	mp := ModelPrice{Model: p.ModelName, Group: g.Name}
	if p.QuotaType == 1 {
		mp.PerRequest = f64(p.ModelPrice * g.Ratio)
		return mp
	}
	in := p.ModelRatio * usdPerMillionAtRatio1 * g.Ratio
	mp.ModelRatio = f64(p.ModelRatio)
	mp.Input = f64(in)
	mp.Output = f64(in * p.CompletionRatio)
	if p.CacheRatio != nil {
		mp.CacheRead = f64(in * *p.CacheRatio)
	}
	if p.CreateCacheRatio != nil {
		mp.CacheWrite = f64(in * *p.CreateCacheRatio)
	}
	return mp
}

// selfGroups reads /api/user/self/groups ({name:{ratio,desc}}); non-numeric ratios ("auto") are skipped.
func selfGroups(ctx context.Context, s Site) ([]GroupRatio, error) {
	c, err := newSession(ctx, s)
	if err != nil {
		return nil, err
	}
	var raw map[string]struct {
		Ratio json.RawMessage `json:"ratio"`
		Desc  string          `json:"desc"`
	}
	if err := c.get(ctx, "/api/user/self/groups", &raw); err != nil {
		return nil, err
	}
	var out []GroupRatio
	for name, g := range raw {
		var r float64
		if json.Unmarshal(g.Ratio, &r) != nil {
			continue
		}
		out = append(out, GroupRatio{Name: name, Ratio: r, Desc: g.Desc})
	}
	return out, nil
}

func (newAPIAdapter) Pricing(ctx context.Context, s Site) (Pricing, error) {
	var resp struct {
		envelopeHead
		Data       []pricingItem      `json:"data"`
		GroupRatio map[string]float64 `json:"group_ratio"`
	}
	err := getJSON(ctx, s.client(), joinURL(s.BaseURL, "/api/pricing"), nil, &resp)
	if err != nil {
		return Pricing{}, fmt.Errorf("pricing: %w", err)
	}
	if err := resp.check(); err != nil {
		return Pricing{}, fmt.Errorf("pricing: %w", err)
	}
	groups, gerr := selfGroups(ctx, s)
	if gerr != nil || len(groups) == 0 {
		groups = groups[:0]
		for n, r := range resp.GroupRatio {
			groups = append(groups, GroupRatio{Name: n, Ratio: r})
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })

	out := Pricing{Groups: groups, Models: []ModelPrice{}}
	for _, p := range resp.Data {
		for _, g := range groups {
			if p.inGroup(g.Name) {
				out.Models = append(out.Models, modelPriceFor(p, g))
			}
		}
	}
	return out, nil
}

func (newAPIAdapter) Announcements(ctx context.Context, s Site) (Announcement, error) {
	return fetchNotice(ctx, s, true)
}

// fetchNotice reads /api/notice. NewAPI wraps it in UserAuth, so credentials are used when usable.
func fetchNotice(ctx context.Context, s Site, withAuth bool) (Announcement, error) {
	c := &naSession{hc: s.client(), base: s.BaseURL, hdr: map[string]string{}}
	if withAuth {
		if sess, err := newSession(ctx, s); err == nil {
			c = sess
		}
	}
	var text string
	if err := c.get(ctx, "/api/notice", &text); err != nil {
		return Announcement{}, fmt.Errorf("notice: %w", err)
	}
	return Announcement{Content: strings.TrimSpace(text)}, nil
}
