package vendorprice

import (
	"errors"
	"math"
	"testing"

	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

func fp(v float64) *float64 { return &v }

func near(a, b float64) bool { return math.Abs(a-b) <= 1e-9 }

func ch(id int, status int, prio int64, weight uint, cost *float64) ChannelInput {
	return ChannelInput{ID: id, Name: "c", Status: status, Priority: prio, Weight: weight, ChannelCost: cost}
}

func TestReferenceCostTopPriorityLayerOnly(t *testing.T) {
	ref := ReferenceCost([]ChannelInput{
		ch(1, ports.ChannelEnabled, 5, 1, fp(0.1)),
		ch(2, ports.ChannelEnabled, 10, 1, fp(0.3)),
		ch(3, ports.ChannelEnabled, 10, 1, fp(0.5)),
	})
	if ref == nil || ref.Priority != 10 || len(ref.Channels) != 2 || !near(ref.Cost, 0.4) {
		t.Fatalf("got %+v", ref)
	}
	if ref.Channels[0].ID != 2 || ref.Channels[1].ID != 3 {
		t.Fatalf("channels not sorted by id: %+v", ref.Channels)
	}
}

func TestReferenceCostWeighted(t *testing.T) {
	ref := ReferenceCost([]ChannelInput{
		ch(1, ports.ChannelEnabled, 1, 3, fp(0.2)),
		ch(2, ports.ChannelEnabled, 1, 1, fp(0.6)),
	})
	if ref == nil || !near(ref.Cost, (3*0.2+1*0.6)/4) {
		t.Fatalf("weighted: %+v", ref)
	}
}

func TestReferenceCostAllZeroWeightsIsSimpleAverage(t *testing.T) {
	ref := ReferenceCost([]ChannelInput{
		ch(1, ports.ChannelEnabled, 1, 0, fp(0.2)),
		ch(2, ports.ChannelEnabled, 1, 0, fp(0.6)),
	})
	if ref == nil || !near(ref.Cost, 0.4) {
		t.Fatalf("simple average: %+v", ref)
	}
}

func TestReferenceCostZeroWeightChannelIgnoredWhenOthersHaveWeight(t *testing.T) {
	ref := ReferenceCost([]ChannelInput{
		ch(1, ports.ChannelEnabled, 1, 0, fp(0.9)),
		ch(2, ports.ChannelEnabled, 1, 2, fp(0.2)),
	})
	if ref == nil || !near(ref.Cost, 0.2) {
		t.Fatalf("got %+v", ref)
	}
}

func TestReferenceCostSkipsDisabledAndUnpriced(t *testing.T) {
	// the disabled channel has the top priority but must not count; the unpriced one neither
	ref := ReferenceCost([]ChannelInput{
		ch(1, ports.ChannelManualDisabled, 99, 1, fp(0.9)),
		ch(2, ports.ChannelAutoDisabled, 99, 1, fp(0.9)),
		ch(3, ports.ChannelEnabled, 50, 1, nil),
		ch(4, ports.ChannelEnabled, 7, 1, fp(0.25)),
	})
	if ref == nil || ref.Priority != 7 || !near(ref.Cost, 0.25) || len(ref.Channels) != 1 {
		t.Fatalf("got %+v", ref)
	}
}

func TestReferenceCostNoCandidates(t *testing.T) {
	if ReferenceCost(nil) != nil {
		t.Fatal("nil input")
	}
	if ReferenceCost([]ChannelInput{ch(1, ports.ChannelManualDisabled, 1, 1, fp(0.1)), ch(2, ports.ChannelEnabled, 1, 1, nil)}) != nil {
		t.Fatal("no enabled priced channel")
	}
	if ReferenceCost([]ChannelInput{ch(1, ports.ChannelEnabled, 1, 1, fp(math.NaN())), ch(2, ports.ChannelEnabled, 1, 1, fp(0))}) != nil {
		t.Fatal("NaN / zero cost must not count")
	}
}

func TestTargetSellPrice(t *testing.T) {
	ref := &Reference{Cost: 0.0243055556}
	if got := TargetSellPrice(store.PricingFixed, fp(0.3), fp(2), ref); got == nil || *got != 0.3 {
		t.Fatalf("fixed: %v", got)
	}
	if got := TargetSellPrice(store.PricingFixed, nil, fp(2), ref); got != nil {
		t.Fatalf("fixed unset: %v", *got)
	}
	// follow: reference × markup, 4 decimals
	if got := TargetSellPrice(store.PricingFollow, nil, fp(1.5), ref); got == nil || *got != 0.0365 {
		t.Fatalf("follow: %v", got)
	}
	if TargetSellPrice(store.PricingFollow, fp(0.3), nil, ref) != nil {
		t.Fatal("follow without markup")
	}
	if TargetSellPrice(store.PricingFollow, fp(0.3), fp(1.5), nil) != nil {
		t.Fatal("follow without reference")
	}
	if TargetSellPrice(store.PricingFollow, nil, fp(0.001), &Reference{Cost: 0.01}) != nil {
		t.Fatal("price that rounds to 0 is not valid")
	}
}

func TestGroupRatioFormula(t *testing.T) {
	r, err := GroupRatio(0.3, 7.2, 7)
	if err != nil || r != 0.309 {
		t.Fatalf("got %v %v", r, err)
	}
	// 站点汇率 1：倍率是站点汇率 7 时的 7 倍（人民币价格不变）
	r1, _ := GroupRatio(0.3, 7.2, 1)
	if r1 != 2.16 {
		t.Fatalf("site rate 1: %v", r1)
	}
	for name, args := range map[string][3]float64{
		"zero price":     {0, 7.2, 7},
		"negative price": {-1, 7.2, 7},
		"NaN price":      {math.NaN(), 7.2, 7},
		"Inf price":      {math.Inf(1), 7.2, 7},
		"zero usd":       {0.3, 0, 7},
		"zero site":      {0.3, 7.2, 0},
		"negative site":  {0.3, 7.2, -7},
		"NaN site":       {0.3, 7.2, math.NaN()},
		"rounds to 0":    {1e-9, 7.2, 7},
	} {
		if _, err := GroupRatio(args[0], args[1], args[2]); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestGroupRatioRoundsToThreeDecimals(t *testing.T) {
	for _, tc := range []struct{ sell, want float64 }{
		{1.2346, 1.235}, {1.2344, 1.234}, {0.0006, 0.001}, {0.309, 0.309}, {0.3085714, 0.309},
	} {
		if r, err := GroupRatio(tc.sell, 1, 1); err != nil || r != tc.want {
			t.Errorf("GroupRatio(%v) = %v, %v; want %v", tc.sell, r, err, tc.want)
		}
	}
	// a price whose ratio rounds to 0 is an error, never a silent 0
	for _, sell := range []float64{0.0004999, 0.0001, 1e-9} {
		if r, err := GroupRatio(sell, 1, 1); !errors.Is(err, ErrRatioRoundsToZero) || r != 0 {
			t.Errorf("GroupRatio(%v) = %v, %v; want ErrRatioRoundsToZero", sell, r, err)
		}
	}
	// the typed ratio survives ratio → price → ratio exactly
	for _, ratio := range []float64{0.001, 0.286, 0.309, 1, 2.16, 7.777} {
		price := PriceFromRatio(ratio, 7.2, 7)
		if got, err := GroupRatio(price, 7.2, 7); err != nil || got != ratio {
			t.Errorf("round trip %v: got %v, %v", ratio, got, err)
		}
	}
	if SyncTolerance < 5e-4 || SyncTolerance > 5.1e-4 || DriftTolerance != 1e-6 {
		t.Errorf("tolerances: %v %v", SyncTolerance, DriftTolerance)
	}
}

func TestClassifyPriority(t *testing.T) {
	base := Facts{
		Mode: store.PricingFollow, Configured: true, HasRef: true, Target: fp(0.3), TargetRatio: fp(0.3),
		NewAPIRatio: fp(0.2), LastRatio: fp(0.2), Change: fp(0.1), ProtectLimit: 0.5,
	}
	tests := []struct {
		name string
		mod  func(*Facts)
		want string
	}{
		{"to write", func(f *Facts) {}, store.PricingToWrite},
		{"synced", func(f *Facts) { f.NewAPIRatio, f.LastRatio = fp(0.3), fp(0.3) }, store.PricingSynced},
		{"synced within half step", func(f *Facts) { f.NewAPIRatio, f.LastRatio = fp(0.3000003), fp(0.3000003) }, store.PricingSynced},
		{"unset beats everything", func(f *Facts) { f.Configured = false; f.HasRef = false; f.NewAPIRatio = fp(9) }, store.PricingUnset},
		{"no channel beats drift", func(f *Facts) { f.HasRef = false; f.NewAPIRatio = fp(9) }, store.PricingNoChannel},
		{"fixed mode does not need a reference", func(f *Facts) { f.Mode = store.PricingFixed; f.HasRef = false }, store.PricingToWrite},
		{"drift beats pending", func(f *Facts) { f.NewAPIRatio = fp(0.9); f.Change = fp(2) }, store.PricingDrift},
		{"drift when group missing in NewAPI", func(f *Facts) { f.NewAPIRatio = nil }, store.PricingDrift},
		{"drift tolerance 1e-6 is not drift", func(f *Facts) { f.NewAPIRatio = fp(0.2000009) }, store.PricingToWrite},
		{"pending beats to write", func(f *Facts) { f.Change = fp(0.51) }, store.PricingPending},
		{"exactly at threshold is not pending", func(f *Facts) { f.Change = fp(0.5) }, store.PricingToWrite},
		{"fixed mode is never pending", func(f *Facts) { f.Mode = store.PricingFixed; f.Change = fp(5) }, store.PricingToWrite},
		{"first write: no NewAPI ratio, no last", func(f *Facts) { f.NewAPIRatio, f.LastRatio, f.Change = nil, nil, nil }, store.PricingToWrite},
		{"no target ratio", func(f *Facts) { f.TargetRatio = nil }, store.PricingUnset},
		{"takeover of a non-3-decimal ratio is synced", func(f *Facts) {
			f.NewAPIRatio, f.LastRatio, f.TargetRatio = fp(0.2857), fp(0.2857), fp(0.286)
		}, store.PricingSynced},
		{"half step boundary is synced", func(f *Facts) {
			f.NewAPIRatio, f.LastRatio, f.TargetRatio = fp(0.2855), fp(0.2855), fp(0.286)
		}, store.PricingSynced},
		{"beyond half step is to write", func(f *Facts) {
			f.NewAPIRatio, f.LastRatio, f.TargetRatio = fp(0.2854), fp(0.2854), fp(0.286)
		}, store.PricingToWrite},
		{"drift tolerance is still 1e-6", func(f *Facts) {
			f.NewAPIRatio, f.LastRatio, f.TargetRatio = fp(0.2857), fp(0.285702), fp(0.2857)
		}, store.PricingDrift},
		{"drift is not softened by the sync tolerance", func(f *Facts) {
			f.NewAPIRatio, f.LastRatio, f.TargetRatio = fp(0.3002), fp(0.3), fp(0.3)
		}, store.PricingDrift},
	}
	for _, tc := range tests {
		f := base
		tc.mod(&f)
		if got := Classify(f); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestEvaluateChangeSmallAndPending(t *testing.T) {
	in := Input{
		Mode: store.PricingFollow, Markup: fp(1), Ref: &Reference{Cost: 0.1}, USDRate: 7.2, SiteRate: 7,
		ProtectLimit: 0.5, MinChange: 0.02, NewAPIRatio: fp(0.1), LastPrice: fp(0.0972), LastRatio: fp(0.1),
	}
	r := Evaluate(in)
	if r.State != store.PricingToWrite || r.Change == nil || !near(*r.Change, (0.1-0.0972)/0.0972) || r.SmallChange {
		t.Fatalf("normal: %+v", r)
	}
	if r.TargetRatio == nil || *r.TargetRatio != 0.103 {
		t.Fatalf("target ratio: %v", r.TargetRatio)
	}

	in.LastPrice = fp(0.0995) // 0.5% away
	if r = Evaluate(in); !r.SmallChange || r.State != store.PricingToWrite {
		t.Fatalf("small change: %+v", r)
	}

	in.LastPrice = fp(0.04) // +150%
	r = Evaluate(in)
	if r.State != store.PricingPending || r.Pending == nil || *r.Pending != 0.1 || r.SignedChange == nil || *r.SignedChange < 1.49 {
		t.Fatalf("pending: %+v", r)
	}

	in.LastPrice = nil
	if r = Evaluate(in); r.Change != nil || r.SmallChange {
		t.Fatalf("no last price: %+v", r)
	}
}
