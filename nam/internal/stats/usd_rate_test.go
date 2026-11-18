package stats

import (
	"testing"
	"time"

	"github.com/steven52880/nam/internal/store"
)

// The 美元参考汇率 divides every cost price (and only those); price tiers do not change.
func TestUSDRate_scales_cost_prices_not_tiers(t *testing.T) {
	m := newModelEnv(t)
	h := hourStart(time.Now().Unix()) - 3600
	must(t, m.st.DB.Create(&store.StatsHourly{
		ChannelID: m.ch1, Model: "other", Hour: h, Success: 4,
		CostCNY: 0.7, RevenueCNY: 2, OfficialUSD: 2, OfficialNoCacheUSD: 4,
	}).Error)
	for _, id := range []int{m.ch2, m.ch3} {
		must(t, m.st.DB.Create(&store.StatsHourly{ChannelID: id, Model: "other", Hour: h, Success: 1}).Error)
	}

	type snap struct {
		std, meas, eff float64
		cost, rev      float64
		tiers          [3]string
	}
	take := func(rate string) snap {
		t.Helper()
		if rate != "" {
			must(t, m.st.SetKV(m.ctx, store.KeyUSDCNYRate, rate))
		}
		got := m.stats(t, Window3d, "")
		it := got[m.ch1]
		return snap{
			std: f(it.StdCostPrice), meas: f(it.MeasuredCostPrice), eff: f(it.EffectiveCostPrice),
			cost: it.CostCNY, rev: it.RevenueCNY,
			tiers: [3]string{tierOf(got[m.ch1]), tierOf(got[m.ch2]), tierOf(got[m.ch3])},
		}
	}

	one := take("1")
	if !approx(one.meas, 0.35) || !approx(one.eff, 0.175) || !approx(one.std, 0.175) {
		t.Fatalf("rate 1: %+v", one)
	}
	for _, rate := range []struct {
		set  string
		want float64
	}{
		{"7.2", 7.2},
		{"", 7.2}, // unchanged: still 7.2
		{"3.6", 3.6},
		{"abc", 7.2},  // unparsable → default
		{"0", 7.2},    // non-positive → default
		{"-2", 7.2},   // non-positive → default
		{"+Inf", 7.2}, // not finite → default
	} {
		got := take(rate.set)
		if !approx(got.meas, one.meas/rate.want) || !approx(got.eff, one.eff/rate.want) || !approx(got.std, one.std/rate.want) {
			t.Fatalf("rate %q: %+v want 1/%v of %+v", rate.set, got, rate.want, one)
		}
		if got.cost != one.cost || got.rev != one.rev || got.tiers != one.tiers {
			t.Fatalf("rate %q changed 元 values or tiers: %+v vs %+v", rate.set, got, one)
		}
	}

	// the per-channel models endpoint is scaled by the same rate
	must(t, m.st.SetKV(m.ctx, store.KeyUSDCNYRate, "2"))
	ms, err := m.svc.ChannelModels(m.ctx, m.ch1, Window3d)
	must(t, err)
	if len(ms.Items) != 1 || !approx(f(ms.Items[0].MeasuredCostPrice), 0.35/2) {
		t.Fatalf("models: %+v", ms.Items)
	}
}
