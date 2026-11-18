package vendorprice

import (
	"reflect"
	"testing"
)

var testVendors = []VendorRef{{Name: "Anthropic", Group: "claude"}, {Name: "OpenAI", Group: "openai"}}

func TestParseOptionsInvalidJSONIsAnError(t *testing.T) {
	for name, raw := range map[string]map[string]string{
		"GroupRatio garbage":        {KeyGroupRatio: "{oops"},
		"GroupRatio array":          {KeyGroupRatio: "[1,2]"},
		"GroupRatio string value":   {KeyGroupRatio: `{"a":"x"}`},
		"UserUsableGroups array":    {KeyUserUsableGroups: `["a"]`},
		"UserUsableGroups num":      {KeyUserUsableGroups: `{"a":1}`},
		"AutoGroups object":         {KeyAutoGroups: `{"a":1}`},
		"AutoGroups non-string":     {KeyAutoGroups: `[1,2]`},
		"GroupRatio trailing stuff": {KeyGroupRatio: `{"a":1} x`},
	} {
		_, errs := ParseOptions(raw)
		if len(errs) != 1 {
			t.Errorf("%s: errs=%v", name, errs)
		}
	}
	opts, errs := ParseOptions(map[string]string{})
	if len(errs) != 0 || opts.GroupRatio == nil || opts.UserUsableGroups == nil || opts.AutoGroups == nil {
		t.Fatalf("missing options must parse as empty: %+v %v", opts, errs)
	}
	opts, errs = ParseOptions(map[string]string{KeyGroupRatio: "null", KeyAutoGroups: " "})
	if len(errs) != 0 || len(opts.GroupRatio) != 0 || len(opts.AutoGroups) != 0 {
		t.Fatalf("null / blank: %+v %v", opts, errs)
	}
}

func TestMergeGroupRatioKeepsOtherKeys(t *testing.T) {
	cur := map[string]float64{"default": 1, "vip": 2, "claude": 0.5}
	got := MergeGroupRatio(cur, map[string]float64{"claude": 0.3, "openai": 0.4})
	want := map[string]float64{"default": 1, "vip": 2, "claude": 0.3, "openai": 0.4}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
	if cur["claude"] != 0.5 || len(cur) != 3 {
		t.Fatal("input must not be mutated")
	}
}

func TestMergeUserUsableGroups(t *testing.T) {
	cur := map[string]string{"default": "默认", "claude": "我自己写的描述"}
	got := MergeUserUsableGroups(cur, testVendors)
	want := map[string]string{"default": "默认", "claude": "我自己写的描述", "openai": "OpenAI", "auto": "自动选择"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
	cur["auto"] = "Auto"
	if got = MergeUserUsableGroups(cur, nil); got["auto"] != "Auto" {
		t.Fatalf("existing auto description must stay: %v", got)
	}
}

func TestMergeAutoGroupsOrder(t *testing.T) {
	cases := []struct {
		name string
		cur  []string
		want []string
	}{
		{"empty", nil, []string{"claude", "openai"}},
		{"vendors first then others in original order", []string{"default", "openai", "vip", "claude", "default"}, []string{"claude", "openai", "default", "vip"}},
		{"already right", []string{"claude", "openai", "default"}, []string{"claude", "openai", "default"}},
	}
	for _, tc := range cases {
		if got := MergeAutoGroups(tc.cur, testVendors); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestMarshalRoundTripAndEquality(t *testing.T) {
	gr := map[string]float64{"b": 0.308571, "a": 1}
	raw := MarshalGroupRatio(gr)
	if raw != `{"a":1,"b":0.308571}` {
		t.Fatalf("raw: %s", raw)
	}
	back, err := ParseGroupRatio(raw)
	if err != nil || !EqualGroupRatio(gr, back) {
		t.Fatalf("round trip: %v %v", back, err)
	}
	// equality is by parsed value: key order and spacing do not matter
	a, _ := ParseGroupRatio(`{"a": 1, "b": 0.308571}`)
	b, _ := ParseGroupRatio(`{"b":0.308571,"a":1.0}`)
	if !EqualGroupRatio(a, b) {
		t.Fatal("should be equal")
	}
	if EqualGroupRatio(a, map[string]float64{"a": 1}) || EqualGroupRatio(a, map[string]float64{"a": 1, "b": 0.3}) {
		t.Fatal("should differ")
	}
	if MarshalAutoGroups(nil) != "[]" || MarshalUserUsableGroups(nil) != "{}" || MarshalGroupRatio(nil) != "{}" {
		t.Fatal("nil must marshal as an empty container")
	}
	if !EqualAutoGroups([]string{"a", "b"}, []string{"a", "b"}) || EqualAutoGroups([]string{"a", "b"}, []string{"b", "a"}) {
		t.Fatal("AutoGroups equality is ordered")
	}
}
