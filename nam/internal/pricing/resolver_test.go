package pricing

import (
	"testing"

	"github.com/steven52880/nam/internal/store"
)

func u1() *uint { v := uint(1); return &v }

// book: test-model in 3 / out 15; m2 in 10; m3 in 1; per-req per-request 0.04; alias claude-x → test-model.
func resolverFixture(t *testing.T) *Book {
	t.Helper()
	st := openStore(t)
	seedBook(t, st)
	st.DB.Create(&store.OfficialPrice{Model: "m2", Input: f(10)})
	st.DB.Create(&store.OfficialPrice{Model: "m3", Input: f(1)})
	st.DB.Create(&store.OfficialPrice{Model: "per-req", PerRequest: f(0.04)})
	st.DB.Create(&store.ModelAlias{Alias: "claude-x", Official: "test-model"})
	b, err := LoadBook(t.Context(), st)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestChannelSources(t *testing.T) {
	b := resolverFixture(t)
	prices := []store.UpstreamModelPrice{
		{UpstreamID: 1, Group: "a", Model: "test-model", Input: f(1.2)}, // 0.4
		{UpstreamID: 1, Group: "a", Model: "m2", Input: f(4)},           // 0.4
		{UpstreamID: 1, Group: "g", Model: "m2", Input: f(5)},           // only group g has a price for m2 → 0.5
	}
	groups := []store.UpstreamGroup{{UpstreamID: 1, Name: "a", Ratio: 0.45}, {UpstreamID: 1, Name: "z", Ratio: 0.8}, {UpstreamID: 1, Name: "zero", Ratio: 0}}
	r := NewResolver(b, prices, groups)

	// manual wins over everything
	c := r.Channel(ChannelSpec{UpstreamID: u1(), HasToken: true, TokenGroup: "a", Models: []string{"test-model", "m2"}, Manual: f(0.35)})
	if c.Source != SourceManual || c.Discount == nil || *c.Discount != 0.35 || c.Reason != "" {
		t.Fatalf("manual: %+v", c)
	}
	// manual ≤ 0 counts as unset → auto
	c = r.Channel(ChannelSpec{UpstreamID: u1(), HasToken: true, TokenGroup: "a", Models: []string{"test-model", "m2"}, Manual: f(0)})
	if c.Source != SourceAuto {
		t.Fatalf("manual 0: %+v", c)
	}
	// auto: median of the model discounts, rounded
	c = r.Channel(ChannelSpec{UpstreamID: u1(), HasToken: true, TokenGroup: "a", Models: []string{"test-model", "m2"}})
	if c.Source != SourceAuto || !near(*c.Discount, 0.4) || c.GroupRatio == nil || *c.GroupRatio != 0.45 {
		t.Fatalf("auto: %+v", c)
	}
	// group: no derivable model price, group ratio known
	c = r.Channel(ChannelSpec{UpstreamID: u1(), HasToken: true, TokenGroup: "z", Models: []string{"test-model"}})
	if c.Source != SourceGroup || *c.Discount != 0.8 {
		t.Fatalf("group: %+v", c)
	}
	// none, one reason each
	cases := []struct {
		name string
		spec ChannelSpec
		want string
	}{
		{"no upstream", ChannelSpec{Models: []string{"m2"}}, "未关联上游"},
		{"no token", ChannelSpec{UpstreamID: u1(), Models: []string{"m2"}}, "未关联令牌"},
		{"no group", ChannelSpec{UpstreamID: u1(), HasToken: true, Models: []string{"m2"}}, "令牌没有分组"},
		{"unknown group", ChannelSpec{UpstreamID: u1(), HasToken: true, TokenGroup: "nope", Models: []string{"m2"}}, "上游没有这个分组的价格和倍率"},
		{"zero ratio is none", ChannelSpec{UpstreamID: u1(), HasToken: true, TokenGroup: "zero", Models: []string{"m2"}}, "上游没有这个分组的价格和倍率"},
	}
	for _, tc := range cases {
		c := r.Channel(tc.spec)
		if c.Source != SourceNone || c.Discount != nil || c.Reason != tc.want {
			t.Errorf("%s: %+v", tc.name, c)
		}
		if m := c.Model("m2"); m.Source != ModelNone || m.Discount != nil {
			t.Errorf("%s: model must have no discount: %+v", tc.name, m)
		}
	}
	// the token's group decides which prices count (group g only prices m2)
	c = r.Channel(ChannelSpec{UpstreamID: u1(), HasToken: true, TokenGroup: "g", Models: []string{"test-model", "m2"}})
	if c.Source != SourceAuto || !near(*c.Discount, 0.5) {
		t.Fatalf("group g: %+v", c)
	}
}

func TestModelSources(t *testing.T) {
	b := resolverFixture(t)
	prices := []store.UpstreamModelPrice{
		{UpstreamID: 1, Group: "a", Model: "test-model", Input: f(1.2)}, // 0.4
		{UpstreamID: 1, Group: "a", Model: "m2", Input: f(4)},           // 0.4
		{UpstreamID: 1, Group: "a", Model: "m3", Input: f(0.5)},         // 0.5
		{UpstreamID: 1, Group: "a", Model: "per-req", PerRequest: f(0.016)},
	}
	groups := []store.UpstreamGroup{{UpstreamID: 1, Name: "a", Ratio: 0.4}}
	r := NewResolver(b, prices, groups)
	spec := ChannelSpec{UpstreamID: u1(), HasToken: true, TokenGroup: "a",
		Models: []string{"test-model", "m2", "m3", "per-req", "claude-x", "unpriced", "missing-up"}}

	// auto channel (median 0.4): model-level uses the model's own price
	c := r.Channel(spec)
	if c.Source != SourceAuto || !near(*c.Discount, 0.4) {
		t.Fatalf("channel: %+v", c)
	}
	if m := c.Model("m3"); m.Source != ModelOwn || !near(*m.Discount, 0.5) || m.Deviation == nil || !near(*m.Deviation, 0.25) || !m.Deviates {
		t.Fatalf("m3 own + deviates (0.5 vs group 0.4): %+v", m)
	}
	if m := c.Model("m2"); m.Source != ModelOwn || m.Deviates {
		t.Fatalf("m2 equals the group ratio → no deviation: %+v", m)
	}
	// per-request billing divides the per-request prices
	if m := c.Model("per-req"); m.Source != ModelOwn || !near(*m.Discount, 0.4) {
		t.Fatalf("per request: %+v", m)
	}
	// alias: claude-x finds the upstream price of test-model and the official price too
	if m := c.Model("claude-x"); m.Source != ModelOwn || !near(*m.Discount, 0.4) || m.Official != "test-model" {
		t.Fatalf("alias: %+v", m)
	}
	// no official price / no upstream price → falls back to the channel discount (source channel)
	if m := c.Model("unpriced"); m.Source != ModelChannel || !near(*m.Discount, 0.4) || m.Reason != "缺原厂价" {
		t.Fatalf("unpriced: %+v", m)
	}
	// override beats manual beats model
	spec.Overrides = map[string]float64{"m3": 0.2}
	spec.Manual = f(0.35)
	c = r.Channel(spec)
	if m := c.Model("m3"); m.Source != ModelOverride || *m.Discount != 0.2 {
		t.Fatalf("override: %+v", m)
	}
	if m := c.Model("m2"); m.Source != ModelManual || *m.Discount != 0.35 {
		t.Fatalf("manual: %+v", m)
	}
	// manual channel: the model's own price is still reported (for the highlight)
	if m := c.Model("m3"); m.Own == nil || !near(*m.Own, 0.5) || !m.Deviates {
		t.Fatalf("own under override: %+v", m)
	}

	// group-only channel: models without own price use the group ratio; those with one use theirs
	r2 := NewResolver(b, prices[:1], groups)
	c = r2.Channel(ChannelSpec{UpstreamID: u1(), HasToken: true, TokenGroup: "a", Models: []string{"m2"}})
	if c.Source != SourceGroup { // m2 has no upstream price → no median → group ratio
		t.Fatalf("expected group, got %+v", c)
	}
	if m := c.Model("m2"); m.Source != ModelChannel || *m.Discount != 0.4 {
		t.Fatalf("channel source: %+v", m)
	}
}

func TestDeviationBaselineIsMedianWithoutGroupRatio(t *testing.T) {
	b := resolverFixture(t)
	prices := []store.UpstreamModelPrice{
		{UpstreamID: 1, Group: "a", Model: "test-model", Input: f(1.2)}, // 0.4
		{UpstreamID: 1, Group: "a", Model: "m2", Input: f(4)},           // 0.4
		{UpstreamID: 1, Group: "a", Model: "m3", Input: f(0.405)},       // 0.405 → 1.25% above the median
	}
	r := NewResolver(b, prices, nil)
	c := r.Channel(ChannelSpec{UpstreamID: u1(), HasToken: true, TokenGroup: "a", Models: []string{"test-model", "m2", "m3"}})
	if c.GroupRatio != nil || !near(*c.Median, 0.4) {
		t.Fatalf("%+v", c)
	}
	if m := c.Model("m3"); !m.Deviates {
		t.Fatalf("1.25%% > 1%% must deviate: %+v", m)
	}
	if m := c.Model("m2"); m.Deviates {
		t.Fatalf("%+v", m)
	}
	// the derivation list carries the same flags
	if !c.Derivation.Models[2].Deviates || c.Derivation.Models[0].Deviates {
		t.Fatalf("derivation: %+v", c.Derivation.Models)
	}
}
