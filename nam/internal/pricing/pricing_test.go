package pricing

import (
	"context"
	"math"
	"path/filepath"
	"testing"

	"github.com/steven52880/nam/internal/store"
)

func f(v float64) *float64 { return &v }

func near(a, b float64) bool { return math.Abs(a-b) <= 1e-6*math.Max(1, math.Abs(b)) }

func openStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "nam.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// plan testdata: test-model input 3, output 15, cache read 0.3, 5m write 3.75
func seedBook(t *testing.T, st *store.Store) {
	t.Helper()
	if err := st.DB.Create(&store.OfficialPrice{Model: "test-model", Input: f(3), Output: f(15), CacheRead: f(0.3), CacheWrite5m: f(3.75), Source: "litellm"}).Error; err != nil {
		t.Fatal(err)
	}
}

func TestStandardRequestCost(t *testing.T) {
	p := Price{Input: f(3), Output: f(15), CacheRead: f(0.3), CacheWrite5m: f(3.75)}
	usd := OfficialUSD(p, StandardRequest, CacheRatio{})
	if !near(usd, 0.00996) {
		t.Fatalf("official = %v, want 0.00996", usd)
	}
	if cny := ChannelCNY(usd, 0.35, 0.5); !near(cny, 0.001743) {
		t.Fatalf("channel = %v, want 0.001743", cny)
	}
	// cache override: read at 50% of input
	usd2 := OfficialUSD(p, StandardRequest, CacheRatio{Read: f(0.5)})
	if !near(usd2, (800*3+200*1.5+500*15)/1e6) {
		t.Fatalf("override = %v", usd2)
	}
	// missing cache price → no cache discount
	if v := OfficialUSD(Price{Input: f(3), Output: f(15)}, StandardRequest, CacheRatio{}); !near(v, (1000*3+500*15)/1e6) {
		t.Fatalf("no cache price = %v", v)
	}
	per := Price{PerRequest: f(0.04)}
	if v := OfficialUSD(per, Usage{Requests: 3}, CacheRatio{}); !near(v, 0.12) {
		t.Fatalf("per request = %v", v)
	}
}

func TestResolveOverrideAlias(t *testing.T) {
	st := openStore(t)
	seedBook(t, st)
	ctx := context.Background()
	st.DB.Create(&store.ModelAlias{Alias: "claude-x", Official: "test-model"})
	st.DB.Create(&store.PriceOverride{Model: "pr", PerRequest: true, PerRequestUSD: f(0.02)})
	b, err := LoadBook(ctx, st)
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := b.Resolve("claude-x"); !ok || *p.Input != 3 || p.Model != "test-model" {
		t.Fatalf("alias: %+v %v", p, ok)
	}
	if p, ok := b.Resolve("pr"); !ok || !p.IsPerRequest() || p.Source != "override" {
		t.Fatalf("override: %+v %v", p, ok)
	}
	if _, ok := b.Resolve("nope"); ok {
		t.Fatal("missing must be !ok")
	}
	st.DB.Create(&store.OfficialPrice{Model: "cheap", Input: f(0.1), Output: f(0.4)})
	b, _ = LoadBook(ctx, st)
	if m := b.CheapestModel([]string{"test-model", "cheap", "nope"}); m != "cheap" {
		t.Fatalf("cheapest = %s", m)
	}
	if m := b.CheapestModel([]string{"nope", "nope2"}); m != "nope" {
		t.Fatalf("fallback = %s", m)
	}
}

func TestDeriveMedianAndDeviation(t *testing.T) {
	st := openStore(t)
	seedBook(t, st)
	st.DB.Create(&store.OfficialPrice{Model: "m2", Input: f(10)})
	st.DB.Create(&store.OfficialPrice{Model: "m3", Input: f(1)})
	b, _ := LoadBook(context.Background(), st)
	// upstream: ratio 1.5 × $2 × group 0.4 = 1.2 → 1.2/3 = 0.4 ; m2 4.0/10 = 0.4 ; m3 0.5/1 = 0.5 (deviates 25%)
	d := Derive([]string{"test-model", "m2", "m3", "missing"}, b, map[string]UpstreamPrice{
		"test-model": {Input: f(1.2)}, "m2": {Input: f(4)}, "m3": {Input: f(0.5)},
	})
	if d.Median == nil || !near(*d.Median, 0.4) {
		t.Fatalf("median = %v", d.Median)
	}
	if !d.Models[2].Deviates || d.Models[0].Deviates || d.Models[3].Reason != "缺原厂价" {
		t.Fatalf("models: %+v", d.Models)
	}
	if e := Derive([]string{"test-model"}, b, nil); e.Median != nil || e.Models[0].Reason != "上游无价" {
		t.Fatalf("no upstream: %+v", e)
	}
}

func TestRefreshAutoKeepsManual(t *testing.T) {
	st := openStore(t)
	seedBook(t, st)
	ctx := context.Background()
	up := uint(1)
	st.DB.Create(&store.UpstreamModelPrice{UpstreamID: up, Group: "a", Model: "test-model", Input: f(1.2)})
	st.DB.Create(&store.ChannelExt{ChannelID: 10, UpstreamID: &up, Discount: 1, DiscountSource: store.DiscountAuto})
	st.DB.Create(&store.ChannelExt{ChannelID: 11, UpstreamID: &up, Discount: 0.35, DiscountSource: store.DiscountManual})
	chs := []ChannelInput{
		{ChannelID: 10, UpstreamID: up, Group: "a", Models: []string{"test-model"}},
		{ChannelID: 11, UpstreamID: up, Group: "a", Models: []string{"test-model"}},
	}
	res, changes, err := RefreshAuto(ctx, st, chs)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Updated) != 1 || res.Manual != 1 || len(changes) != 1 || !near(changes[0].New, 0.4) {
		t.Fatalf("res=%+v changes=%+v", res, changes)
	}
	var e10, e11 store.ChannelExt
	st.DB.First(&e10, 10)
	st.DB.First(&e11, 11)
	if !near(e10.Discount, 0.4) || e10.DiscountSource != store.DiscountAuto || e11.Discount != 0.35 {
		t.Fatalf("e10=%+v e11=%+v", e10, e11)
	}
	// second run: no churn
	res, changes, _ = RefreshAuto(ctx, st, chs)
	if len(res.Updated) != 0 || len(changes) != 0 {
		t.Fatalf("churn: %+v", res)
	}
}
