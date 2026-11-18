package stats

import (
	"reflect"
	"testing"
)

func TestWindowSpan_table(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"1h", 3600, true},
		{"12h", 12 * 3600, true},
		{"72h", 72 * 3600, true},
		{"1d", 86400, true},
		{"3d", 72 * 3600, true},
		{"7d", 168 * 3600, true},
		{"30d", 30 * 86400, true},
		{"186d", MaxSpanSeconds, true},
		{"4464h", MaxSpanSeconds, true},
		{"4465h", 0, false},
		{"187d", 0, false},
		{"99999d", 0, false},
		{"999999999999999999999d", 0, false},
		{"0h", 0, false},
		{"0d", 0, false},
		{"-1d", 0, false},
		{"03d", 0, false}, // leading zeros are rejected
		{"1.5d", 0, false},
		{"3D", 0, false},
		{"d", 0, false},
		{"h", 0, false},
		{"3", 0, false},
		{"3w", 0, false},
		{"3 d", 0, false},
		{" 3d", 0, false},
		{"3d ", 0, false},
		{"+3d", 0, false},
		{"latest", 0, false}, // handled by validWindow, not windowSpan
		{"", 0, false},
	}
	for _, c := range cases {
		got, err := windowSpan(c.in)
		if (err == nil) != c.ok || got != c.want {
			t.Errorf("windowSpan(%q) = %d, %v; want %d ok=%v", c.in, got, err, c.want, c.ok)
		}
	}
}

func TestValidWindow(t *testing.T) {
	for _, w := range []string{"latest", "1h", "3d", "7d", "186d"} {
		if err := validWindow(w); err != nil {
			t.Errorf("validWindow(%q) = %v", w, err)
		}
	}
	for _, w := range []string{"", "Latest", "0d", "03d", "187d", "1.5d", "3w"} {
		if err := validWindow(w); err == nil {
			t.Errorf("validWindow(%q) should fail", w)
		}
	}
}

func TestChannelModels_custom_windows(t *testing.T) {
	e := newEnv(t)
	seedRows(t, e) // rows sit one hour before the current hour
	for _, w := range []string{"2h", "12h", "3d", "72h", "30d"} {
		ms, err := e.svc.ChannelModels(e.ctx, 1, w)
		if err != nil || len(ms.Items) != 2 {
			t.Errorf("window %s: %+v %v", w, ms.Items, err)
		}
	}
	a, err := e.svc.ChannelModels(e.ctx, 1, "3d")
	must(t, err)
	b, err := e.svc.ChannelModels(e.ctx, 1, "72h")
	must(t, err)
	if !reflect.DeepEqual(a.Items, b.Items) {
		t.Fatalf("3d %+v != 72h %+v", a.Items, b.Items)
	}
	for _, w := range []string{"187d", "0h", "3w"} {
		if _, err := e.svc.ChannelModels(e.ctx, 1, w); err == nil {
			t.Errorf("window %s should fail", w)
		}
	}
}
