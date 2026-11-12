package vendors

import (
	"context"
	"strings"

	"github.com/steven52880/nam/internal/store"
)

// MatchBy says how a model was assigned to a vendor.
type MatchBy string

const (
	MatchNone     MatchBy = ""
	MatchExplicit MatchBy = "explicit" // model is listed in Vendor.Models
	MatchPrefix   MatchBy = "prefix"   // model starts with one of Vendor.Prefixes
)

type prefixRule struct {
	lower  string // lower-cased prefix used for comparison
	prefix string // prefix as configured
	vendor string
}

// Matcher answers "which vendor owns this model". Build one per vendor-list snapshot.
type Matcher struct {
	explicit map[string]string // model -> vendor name
	prefixes []prefixRule      // sorted by descending prefix length, then vendor order
}

// NewMatcher indexes the vendors (in list order). Explicit models win over prefixes;
// among prefixes the longest wins, so `gpt-image-` beats `gpt-`.
func NewMatcher(vs []store.Vendor) *Matcher {
	m := &Matcher{explicit: map[string]string{}}
	for _, v := range vs {
		for _, name := range parseModels(v.Models) {
			if _, taken := m.explicit[name]; !taken {
				m.explicit[name] = v.Name
			}
		}
		for _, p := range v.Prefixes {
			if p = strings.TrimSpace(p); p != "" {
				m.prefixes = append(m.prefixes, prefixRule{lower: strings.ToLower(p), prefix: p, vendor: v.Name})
			}
		}
	}
	// insertion sort keeps equal-length prefixes in vendor order (stable)
	for i := 1; i < len(m.prefixes); i++ {
		for j := i; j > 0 && len(m.prefixes[j].lower) > len(m.prefixes[j-1].lower); j-- {
			m.prefixes[j], m.prefixes[j-1] = m.prefixes[j-1], m.prefixes[j]
		}
	}
	return m
}

// MatchItem is one model's ownership in the match endpoint response.
type MatchItem struct {
	Model  string  `json:"model"`
	Vendor string  `json:"vendor"`
	By     MatchBy `json:"by"`
	Prefix string  `json:"prefix"`
}

// Match resolves the owner of each model against the current vendor list.
func (s *Service) Match(ctx context.Context, models []string) ([]MatchItem, error) {
	var vs []store.Vendor
	if err := s.St.DB.WithContext(ctx).Order("`order`, id").Find(&vs).Error; err != nil {
		return nil, err
	}
	m := NewMatcher(vs)
	items := make([]MatchItem, 0, len(models))
	for _, name := range models {
		v, by, p := m.Owner(name)
		items = append(items, MatchItem{Model: name, Vendor: v, By: by, Prefix: p})
	}
	return items, nil
}

// Owner returns the owning vendor, how it matched and the matching prefix (prefix matches
// only). An unowned model returns ("", MatchNone, "").
func (m *Matcher) Owner(model string) (vendor string, by MatchBy, prefix string) {
	if v, ok := m.explicit[model]; ok {
		return v, MatchExplicit, ""
	}
	lower := strings.ToLower(model)
	for _, p := range m.prefixes {
		if strings.HasPrefix(lower, p.lower) {
			return p.vendor, MatchPrefix, p.prefix
		}
	}
	return "", MatchNone, ""
}
