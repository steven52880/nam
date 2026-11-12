package vendors

import (
	"context"
	"strings"
	"testing"

	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
)

func TestMatcherLongestPrefixAndExplicit(t *testing.T) {
	m := NewMatcher([]store.Vendor{
		{Name: "openai", Prefixes: []string{"gpt-", "o1"}, Models: `["gpt-image-special"]`},
		{Name: "image", Prefixes: []string{"GPT-Image-"}},
		{Name: "pinned", Models: `["gpt-4o"]`},
	})
	cases := []struct {
		model, vendor string
		by            MatchBy
		prefix        string
	}{
		{"gpt-image-1", "image", MatchPrefix, "GPT-Image-"}, // longest prefix wins, case-insensitive
		{"GPT-4-turbo", "openai", MatchPrefix, "gpt-"},      // model case-insensitive
		{"gpt-4o", "pinned", MatchExplicit, ""},             // explicit beats prefix
		{"gpt-image-special", "openai", MatchExplicit, ""},  // explicit beats longer prefix of another vendor
		{"o1-mini", "openai", MatchPrefix, "o1"},
		{"claude-3", "", MatchNone, ""},
	}
	for _, c := range cases {
		v, by, p := m.Owner(c.model)
		if v != c.vendor || by != c.by || p != c.prefix {
			t.Errorf("%s: got (%q,%q,%q), want (%q,%q,%q)", c.model, v, by, p, c.vendor, c.by, c.prefix)
		}
	}
	if v, by, _ := NewMatcher(nil).Owner("x"); v != "" || by != MatchNone {
		t.Errorf("empty matcher: %q %q", v, by)
	}
}

func TestPrefixValidationAndConflict(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()
	a, err := s.Create(ctx, Input{Name: "openai", GroupName: "openai", Prefixes: []string{" gpt- ", "GPT-", "o1"}})
	if err != nil || len(a.Prefixes) != 2 || a.Prefixes[0] != "gpt-" {
		t.Fatalf("trim+dedupe: %+v %v", a, err)
	}
	_, err = s.Create(ctx, Input{Name: "other", GroupName: "other", Prefixes: []string{"GPT-"}})
	if resp.Status(err) != 409 || !strings.Contains(err.Error(), "openai") {
		t.Errorf("same prefix must 409 naming the other vendor: %v", err)
	}
	b, err := s.Create(ctx, Input{Name: "image", GroupName: "image", Prefixes: []string{"gpt-image-"}})
	if err != nil {
		t.Fatalf("different prefix ok: %v", err)
	}
	if _, err := update(s, b.ID, Input{Name: "image", GroupName: "image", Prefixes: []string{"O1"}}); resp.Status(err) != 409 {
		t.Errorf("update conflict: %v", err)
	}
	if _, err := update(s, a.ID, Input{Name: "openai", GroupName: "openai", Prefixes: []string{"gpt-"}}); err != nil {
		t.Errorf("update keeps own prefix: %v", err)
	}
	items, err := s.Match(ctx, []string{"gpt-image-1", "gpt-4", "zzz"})
	if err != nil || len(items) != 3 || items[0].Vendor != "image" || items[1].Vendor != "openai" ||
		items[1].By != MatchPrefix || items[2].Vendor != "" || items[2].By != MatchNone {
		t.Errorf("match: %+v %v", items, err)
	}
}
