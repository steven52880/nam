package stats

import (
	"testing"
	"time"

	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

// modelEnv: three channels of one vendor, same upstream (rate 0.5).
//
//	ch1 models test-model+other, discount 0.35, test-model override 0.2
//	ch2 models test-model, discount 0.5
//	ch3 models other, discount 0.1
//
// "other" is priced (cheaper than test-model) so ch1 and ch3 are rated on it when no
// model is given.
type modelEnv struct {
	*env
	ch1, ch2, ch3 int
}

func newModelEnv(t *testing.T) *modelEnv {
	t.Helper()
	e := newEnv(t)
	must(t, e.st.DB.Create(&store.OfficialPrice{Model: "other", Input: f64(1), Output: f64(5)}).Error)
	var up store.Upstream
	must(t, e.st.DB.First(&up).Error)
	must(t, e.st.DB.Where("1 = 1").Delete(&store.ChannelExt{}).Error)
	me := &modelEnv{env: e}
	mk := func(name string, disc float64, override string, models ...string) int {
		id, err := e.fake.Create(e.ctx, ports.ChannelCreate{Name: name, Models: models})
		must(t, err)
		must(t, e.st.DB.Create(&store.ChannelExt{ChannelID: id, UpstreamID: &up.ID, Vendor: "ext-vendor", Discount: disc, ModelDiscount: override}).Error)
		return id
	}
	me.ch1 = mk("c1", 0.35, `{"test-model":0.2}`, "test-model", "other")
	me.ch2 = mk("c2", 0.5, "", "test-model")
	me.ch3 = mk("c3", 0.1, "", "other")
	return me
}

func (m *modelEnv) stats(t *testing.T, window, model string) map[int]ChannelStatsItem {
	t.Helper()
	got, err := m.svc.ChannelStats(m.ctx, window, model)
	must(t, err)
	if got.Model != model {
		t.Fatalf("model echo %q want %q", got.Model, model)
	}
	out := map[int]ChannelStatsItem{}
	for _, it := range got.Items {
		out[it.ChannelID] = it
	}
	return out
}

func tierOf(it ChannelStatsItem) string {
	if it.PriceTier == nil {
		return "<nil>"
	}
	return *it.PriceTier
}

func TestChannelStats_model_stored_window(t *testing.T) {
	m := newModelEnv(t)
	h := hourStart(time.Now().Unix()) - 3600
	for _, r := range []store.StatsHourly{
		{ChannelID: m.ch1, Model: "test-model", Success: 5},
		{ChannelID: m.ch1, Model: "other", Success: 7},
		{ChannelID: m.ch2, Model: "test-model", Success: 3},
		{ChannelID: m.ch3, Model: "other", Success: 2},
	} {
		r.Hour = h
		must(t, m.st.DB.Create(&r).Error)
	}

	// no model: channel totals and tiers over all three channels, exactly as before
	all := m.stats(t, Window3d, "")
	if len(all) != 3 || all[m.ch1].Success != 12 || all[m.ch2].Success != 3 || all[m.ch3].Success != 2 {
		t.Fatalf("no model: %+v", all)
	}
	if !approx(f(all[m.ch1].StdCostPrice), 0.175) || !approx(f(all[m.ch3].StdCostPrice), 0.05) {
		t.Fatalf("no model std: %v %v", all[m.ch1].StdCostPrice, all[m.ch3].StdCostPrice)
	}
	if tierOf(all[m.ch3]) != TierCheap || tierOf(all[m.ch1]) != TierMid || tierOf(all[m.ch2]) != TierExpensive {
		t.Fatalf("no model tiers: %s %s %s", tierOf(all[m.ch1]), tierOf(all[m.ch2]), tierOf(all[m.ch3]))
	}

	// model: only channels with data for it; counts are that model's; price taken on that
	// model (ch1's per-model discount 0.2) and tiers compare only ch1 and ch2
	got := m.stats(t, Window3d, "test-model")
	if len(got) != 2 || got[m.ch1].Success != 5 || got[m.ch2].Success != 3 {
		t.Fatalf("test-model: %+v", got)
	}
	if !approx(f(got[m.ch1].StdCostPrice), 0.2*0.5) || !approx(f(got[m.ch2].StdCostPrice), 0.5*0.5) {
		t.Fatalf("test-model std: %v %v", got[m.ch1].StdCostPrice, got[m.ch2].StdCostPrice)
	}
	if tierOf(got[m.ch1]) != TierCheap || tierOf(got[m.ch2]) != TierMid {
		t.Fatalf("test-model tiers: %s %s", tierOf(got[m.ch1]), tierOf(got[m.ch2]))
	}

	// a model nobody has: empty list (not null), echoed back
	none, err := m.svc.ChannelStats(m.ctx, Window3d, "nobody")
	must(t, err)
	if none.Items == nil || len(none.Items) != 0 {
		t.Fatalf("nobody: %+v", none.Items)
	}
}

func TestChannelStats_model_latest_window(t *testing.T) {
	m := newModelEnv(t)
	h := hourStart(time.Now().Unix())
	now := time.Now().Unix()
	for _, id := range []int{m.ch1, m.ch2, m.ch3} {
		must(t, m.st.DB.Create(&store.StatsHourly{ChannelID: id, Model: "x", Hour: h, Success: 1}).Error)
	}
	other := func(ch int, at int64) ports.LogRecord {
		r := stdLog(ch, at)
		r.ModelName = "other"
		return r
	}
	m.fake.AddLog(stdLog(m.ch1, now-60))
	m.fake.AddLog(stdLog(m.ch1, now-50))
	m.fake.AddLog(other(m.ch1, now-40))
	m.fake.AddLog(stdLog(m.ch2, now-60))
	m.fake.AddLog(other(m.ch3, now-60))

	all := m.stats(t, WindowLatest, "")
	if len(all) != 3 || all[m.ch1].Success != 3 {
		t.Fatalf("latest no model: %+v", all)
	}
	got := m.stats(t, WindowLatest, "test-model")
	if len(got) != 2 || got[m.ch1].Success != 2 || got[m.ch2].Success != 1 {
		t.Fatalf("latest test-model: %+v", got)
	}
	if tierOf(got[m.ch1]) != TierCheap || tierOf(got[m.ch2]) != TierMid {
		t.Fatalf("latest tiers: %s %s", tierOf(got[m.ch1]), tierOf(got[m.ch2]))
	}
}

func TestRate_model_restricts_channels(t *testing.T) {
	m := newModelEnv(t)
	pr, err := NewPricer(m.ctx, m.st, m.fake.Ports().Backend)
	must(t, err)
	chans, err := m.svc.Channels.List(m.ctx)
	must(t, err)

	r, err := m.svc.rate(m.ctx, pr, chans, "other")
	must(t, err)
	if _, ok := r.std[m.ch2]; ok {
		t.Fatalf("ch2 does not carry 'other' but was rated: %v", r.std)
	}
	// ch1 has no override for "other": channel discount 0.35; ch3: 0.1
	if !approx(f(r.std[m.ch1]), 0.175) || !approx(f(r.std[m.ch3]), 0.05) {
		t.Fatalf("std: %v %v", r.std[m.ch1], r.std[m.ch3])
	}
	if t1, t3 := r.tierPtr(m.ch1), r.tierPtr(m.ch3); t1 == nil || t3 == nil || *t3 != TierCheap || *t1 != TierMid {
		t.Fatalf("tiers: %v %v", t1, t3)
	}

	// an unpriced model has no standard price
	r, err = m.svc.rate(m.ctx, pr, chans, "nope")
	must(t, err)
	if len(r.std) != 0 || len(r.tier) != 0 {
		t.Fatalf("nope: %+v", r)
	}
	var ch1 ports.Channel
	for _, c := range chans {
		if c.ID == m.ch1 {
			ch1 = c
		}
	}
	if stdPrice(pr, ch1, nil, "unpriced") != nil {
		t.Fatal("stdPrice on an unpriced model must be nil")
	}
	// empty model keeps the old choice (cheapest priced model: other → 0.35 × 0.5)
	if v := stdPrice(pr, ch1, nil, ""); v == nil || !approx(*v, 0.175) {
		t.Fatalf("stdPrice no model: %v", v)
	}
}

func approx(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= 1e-9
}

func f(p *float64) float64 {
	if p == nil {
		return -1
	}
	return *p
}
