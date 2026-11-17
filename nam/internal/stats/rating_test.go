package stats

import "testing"

func wp(v float64) *float64 { return &v }

func TestQualityTier_boundaries(t *testing.T) {
	cases := []struct {
		name string
		n    int64
		w    *float64
		want string
	}{
		{"19 requests is insufficient", 19, wp(1), QualityInsufficient},
		{"20 requests, wilson .99", 20, wp(0.99), QualityGood},
		{"wilson .9899 is mid", 100, wp(0.9899), QualityMid},
		{"wilson .95 is mid", 100, wp(0.95), QualityMid},
		{"wilson .9499 is bad", 100, wp(0.9499), QualityBad},
		{"nil wilson", 100, nil, QualityInsufficient},
	}
	for _, c := range cases {
		if got := qualityTier(c.n, c.w); got != c.want {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
	}
}

func TestPriceTiers_floorThirds(t *testing.T) {
	// Given 6 channels in one vendor, When tiered, Then floor(3i/6): 2 cheap, 2 mid, 2 expensive.
	var es []stdEntry
	for i, p := range []float64{0.6, 0.1, 0.5, 0.2, 0.4, 0.3} {
		es = append(es, stdEntry{id: i + 1, vendor: "v", price: p})
	}
	got := priceTiers(es)
	want := map[int]string{2: TierCheap, 4: TierCheap, 6: TierMid, 5: TierMid, 3: TierExpensive, 1: TierExpensive}
	for id, tier := range want {
		if got[id] != tier {
			t.Errorf("channel %d: got %q want %q", id, got[id], tier)
		}
	}
}

func TestPriceTiers_smallGroups(t *testing.T) {
	got := priceTiers([]stdEntry{
		{id: 1, vendor: "solo", price: 1},
		{id: 2, vendor: "", price: 1}, {id: 3, vendor: "", price: 2},
		{id: 4, vendor: "pair", price: 1}, {id: 5, vendor: "pair", price: 2},
	})
	if _, ok := got[1]; ok {
		t.Error("single-channel vendor must not be tiered")
	}
	if _, ok := got[2]; ok {
		t.Error("vendorless channels must not be tiered")
	}
	// n=2: i=0 -> 0 (cheap), i=1 -> floor(3/2)=1 (mid)
	if got[4] != TierCheap || got[5] != TierMid {
		t.Errorf("pair tiers: %v", got)
	}
}
