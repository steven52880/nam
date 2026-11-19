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

func TestFollowPrice(t *testing.T) {
	ref := &Reference{Cost: 0.0243055556}
	// reference × markup, 4 decimals
	if got := FollowPrice(fp(1.5), ref); got == nil || *got != 0.0365 {
		t.Fatalf("follow: %v", got)
	}
	if FollowPrice(nil, ref) != nil {
		t.Fatal("follow without markup")
	}
	if FollowPrice(fp(1.5), nil) != nil {
		t.Fatal("follow without reference")
	}
	if FollowPrice(fp(0.001), &Reference{Cost: 0.01}) != nil {
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
	if SyncTolerance < 5e-4 || SyncTolerance > 5.1e-4 {
		t.Errorf("tolerance: %v", SyncTolerance)
	}
}

func fixedIn() Input {
	return Input{Mode: store.PricingFixed, USDRate: 7.2, SiteRate: 7, ProtectLimit: 0.5, MinChange: 0.02}
}

func followIn() Input {
	return Input{
		Mode: store.PricingFollow, Markup: fp(1), Ref: &Reference{Cost: 0.1}, USDRate: 7.2, SiteRate: 7,
		ProtectLimit: 0.5, MinChange: 0.02, NewAPIRatio: fp(0.1),
	}
}

func TestEvaluateFixed(t *testing.T) {
	tests := []struct {
		name  string
		mod   func(*Input)
		state string
		ratio *float64
	}{
		{"nothing at all", func(in *Input) {}, store.PricingUnset, nil},
		{"NewAPI value, no draft", func(in *Input) { in.NewAPIRatio = fp(0.3) }, store.PricingSynced, fp(0.3)},
		{"NewAPI not a 3 decimal number stays synced", func(in *Input) { in.NewAPIRatio = fp(0.2857) }, store.PricingSynced, fp(0.2857)},
		{"draft differs", func(in *Input) { in.NewAPIRatio, in.DraftRatio = fp(0.3), fp(0.4) }, store.PricingToWrite, fp(0.4)},
		{"draft without NewAPI group", func(in *Input) { in.DraftRatio = fp(0.4) }, store.PricingToWrite, fp(0.4)},
		{"draft equal to NewAPI", func(in *Input) { in.NewAPIRatio, in.DraftRatio = fp(0.3), fp(0.3) }, store.PricingSynced, fp(0.3)},
		{"draft within half a step", func(in *Input) { in.NewAPIRatio, in.DraftRatio = fp(0.2857), fp(0.286) }, store.PricingSynced, fp(0.286)},
		{"draft beyond half a step", func(in *Input) { in.NewAPIRatio, in.DraftRatio = fp(0.2854), fp(0.286) }, store.PricingToWrite, fp(0.286)},
		{"fixed is never pending", func(in *Input) { in.NewAPIRatio, in.DraftRatio = fp(0.1), fp(5) }, store.PricingToWrite, fp(5)},
		{"invalid NewAPI ratio is ignored", func(in *Input) { in.NewAPIRatio = fp(0) }, store.PricingUnset, nil},
	}
	for _, tc := range tests {
		in := fixedIn()
		tc.mod(&in)
		r := Evaluate(in)
		if r.State != tc.state {
			t.Errorf("%s: state %s, want %s", tc.name, r.State, tc.state)
		}
		if (tc.ratio == nil) != (r.TargetRatio == nil) || (tc.ratio != nil && !near(*r.TargetRatio, *tc.ratio)) {
			t.Errorf("%s: target ratio %v, want %v", tc.name, r.TargetRatio, tc.ratio)
		}
		if r.SmallChange {
			t.Errorf("%s: fixed mode has no small change", tc.name)
		}
	}
	// the price is the target ratio × site rate ÷ usd rate
	in := fixedIn()
	in.NewAPIRatio = fp(0.309)
	if r := Evaluate(in); r.Target == nil || !near(*r.Target, 0.309*7/7.2) {
		t.Fatalf("price: %v", r.Target)
	}
}

func TestEvaluateFollow(t *testing.T) {
	// 0.1 × 1 → ratio 0.103, NewAPI has 0.1 (price 0.0972): +2.9%, above the minimum
	r := Evaluate(followIn())
	if r.State != store.PricingToWrite || r.Change == nil || !near(*r.Change, (0.1-0.1*7/7.2)/(0.1*7/7.2)) || r.SmallChange {
		t.Fatalf("normal: %+v", r)
	}
	if r.TargetRatio == nil || *r.TargetRatio != 0.103 {
		t.Fatalf("target ratio: %v", r.TargetRatio)
	}

	// NewAPI edited by hand: nam wins, the vendor is to_write again (still protected)
	in := followIn()
	in.NewAPIRatio = fp(0.105)
	if r = Evaluate(in); r.State != store.PricingToWrite || r.Change == nil || *r.Change >= 0 {
		t.Fatalf("hand edit: %+v", r)
	}

	// small change: below the minimum, still to_write
	in = followIn()
	in.NewAPIRatio = fp(0.1022)
	if r = Evaluate(in); !r.SmallChange || r.State != store.PricingToWrite {
		t.Fatalf("small change: %+v", r)
	}

	// already there
	in = followIn()
	in.NewAPIRatio = fp(0.103)
	if r = Evaluate(in); r.State != store.PricingSynced {
		t.Fatalf("synced: %+v", r)
	}

	// +150% (NewAPI is far below): pending; a decrease beyond the limit is pending too
	in = followIn()
	in.NewAPIRatio = fp(0.04)
	if r = Evaluate(in); r.State != store.PricingPending || r.Change == nil || *r.Change < 1.49 {
		t.Fatalf("pending up: %+v", r)
	}
	in.NewAPIRatio = fp(0.5)
	if r = Evaluate(in); r.State != store.PricingPending || r.Change == nil || *r.Change > -0.5 {
		t.Fatalf("pending down: %+v", r)
	}

	// no NewAPI group: to_write, no change
	in = followIn()
	in.NewAPIRatio = nil
	if r = Evaluate(in); r.State != store.PricingToWrite || r.Change != nil || r.SmallChange {
		t.Fatalf("no group: %+v", r)
	}

	// unset and no_channel
	in = followIn()
	in.Markup = nil
	if r = Evaluate(in); r.State != store.PricingUnset {
		t.Fatalf("no markup: %+v", r)
	}
	in = followIn()
	in.Ref = nil
	if r = Evaluate(in); r.State != store.PricingNoChannel {
		t.Fatalf("no reference: %+v", r)
	}
	in = followIn()
	in.Markup, in.Ref = nil, nil
	if r = Evaluate(in); r.State != store.PricingUnset {
		t.Fatalf("unset beats no channel: %+v", r)
	}

	// a price whose ratio rounds to 0 is unset (never written)
	in = followIn()
	in.Ref = &Reference{Cost: 0.0004}
	if r = Evaluate(in); r.State != store.PricingUnset || r.Target == nil || r.TargetRatio != nil {
		t.Fatalf("rounds to zero: %+v", r)
	}
}
