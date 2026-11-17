package stats

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/steven52880/nam/ports"
)

func TestHist_quantile_returns_bucket_midpoint_within_5_percent(t *testing.T) {
	h := Hist{}
	for i := 1; i <= 100; i++ {
		h.Add(float64(i * 10)) // 10..1000
	}
	p50, ok := h.Quantile(0.5)
	if !ok || math.Abs(p50-500)/500 > 0.05 {
		t.Fatalf("p50 %v", p50)
	}
	p95, _ := h.Quantile(0.95)
	if math.Abs(p95-950)/950 > 0.05 {
		t.Fatalf("p95 %v", p95)
	}
}

func TestHist_single_value_and_empty(t *testing.T) {
	h := Hist{}
	if _, ok := h.Quantile(0.5); ok {
		t.Fatal("empty must not be ok")
	}
	h.Add(0)
	h.Add(-3)
	if h.Count() != 0 {
		t.Fatal("non-positive values must be dropped")
	}
	h.Add(300)
	v, _ := h.Quantile(0.95)
	if math.Abs(v-300)/300 > 0.045 {
		t.Fatalf("single %v", v)
	}
}

func TestHist_encode_parse_merge_round_trip(t *testing.T) {
	a := Hist{}
	a.Add(300)
	a.Add(300)
	b := ParseHist(a.Encode())
	b.Merge(a)
	if b.Count() != 4 || len(b) != 1 {
		t.Fatalf("merged %v", b)
	}
	var m map[string]int64
	if err := json.Unmarshal([]byte(b.Encode()), &m); err != nil || len(m) != 1 {
		t.Fatalf("encoded %q", b.Encode())
	}
	if ParseHist("garbage").Count() != 0 || Hist(nil).Encode() != "" {
		t.Fatal("garbage / empty")
	}
}

func TestWilson_fixed_values(t *testing.T) {
	if v := Wilson(10, 10); math.Abs(v-0.722) > 0.001 {
		t.Fatalf("10/10 = %v", v)
	}
	if v := Wilson(990, 1000); math.Abs(v-0.9817) > 0.0001 {
		t.Fatalf("990/1000 = %v", v)
	}
}

func TestMetrics_standard_request_fixed_numbers(t *testing.T) {
	e := newEnv(t)
	pr, err := NewPricer(e.ctx, e.st, e.svc.Backend)
	must(t, err)
	s := mustParse(t, withID(stdLog(1, 3600), 1))
	p := pr.Price(s)
	if !p.OK || !near(p.Official, 0.00996, 1e-12) || !near(p.OfficialNoCache, 0.0105, 1e-12) || !near(p.Cost, 0.001743, 1e-12) {
		t.Fatalf("priced %+v", p)
	}
	if !near(p.Revenue, 1743.0/500000*7, 1e-12) {
		t.Fatalf("revenue %v", p.Revenue)
	}
	a := NewAgg()
	a.Add(s, p)
	m := a.Metrics()
	if !near(*m.MeasuredCostPrice, 0.175, 1e-9) || !near(*m.EffectiveCostPrice, 0.001743/0.0105, 1e-9) {
		t.Fatalf("cost prices %v %v", *m.MeasuredCostPrice, *m.EffectiveCostPrice)
	}
	if !near(*m.CacheHit, 0.2, 1e-12) || m.SpeedEstimated || m.PricedRequests != 1 || m.Requests != 1 {
		t.Fatalf("metrics %+v", m)
	}
	if math.Abs(*m.SpeedP50-250)/250 > 0.05 || math.Abs(*m.FRTP50-300)/300 > 0.05 {
		t.Fatalf("speed %v frt %v", *m.SpeedP50, *m.FRTP50)
	}
}

func TestMetrics_empty_is_null(t *testing.T) {
	m := NewAgg().Metrics()
	if m.SuccessRate != nil || m.Wilson != nil || m.CacheHit != nil || m.FRTP50 != nil || m.SpeedP50 != nil ||
		m.EffectiveCostPrice != nil || m.MeasuredCostPrice != nil || m.Margin != nil {
		t.Fatalf("expected nulls: %+v", m)
	}
}

func TestMetrics_estimated_speed_only_without_exact_samples(t *testing.T) {
	a := NewAgg()
	a.Add(Sample{OK: true, Speed: 100, SpeedEst: true}, Priced{})
	if m := a.Metrics(); !m.SpeedEstimated || m.SpeedP50 == nil {
		t.Fatalf("estimated only: %+v", m)
	}
	a.Add(Sample{OK: true, Speed: 250}, Priced{})
	if m := a.Metrics(); m.SpeedEstimated || math.Abs(*m.SpeedP50-250)/250 > 0.05 {
		t.Fatalf("exact wins: %+v", m)
	}
}

func TestAgg_failures_and_margin(t *testing.T) {
	a := NewAgg()
	a.Add(Sample{FailClass: Class5xx}, Priced{})
	a.Add(Sample{OK: true, Quota: 1}, Priced{OK: true, Cost: 1, Revenue: 4, Official: 2, OfficialNoCache: 4})
	b := NewAgg()
	b.Merge(a)
	m := b.Metrics()
	if m.Requests != 2 || m.Fail != 1 || m.FailByClass[Class5xx] != 1 || *m.SuccessRate != 0.5 || *m.Margin != 0.75 {
		t.Fatalf("metrics %+v", m)
	}
}

func withID(r ports.LogRecord, id int64) ports.LogRecord {
	r.ID = id
	return r
}
