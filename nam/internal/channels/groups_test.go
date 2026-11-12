package channels

import (
	"reflect"
	"testing"
)

func TestSplitGroups(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{" , ", nil},
		{"a", []string{"a"}},
		{" a , ,b,a ", []string{"a", "b"}},
	}
	for _, c := range cases {
		if got := splitGroups(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitGroups(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestMergeManaged(t *testing.T) {
	managed := func(g string) bool { return g == "claude" || g == "gemini" || g == "old" || g == "new" }
	cases := []struct {
		name        string
		live, draft string
		want        string
	}{
		{"keeps unmanaged", "default,claude", "claude", "default,claude"},
		{"draft adds managed", "default", "default,gemini", "default,gemini"},
		{"draft removes managed", "default,claude,vip", "", "default,vip"},
		{"rename keeps position", "a,old,b", "new", "a,new,b"},
		{"live manual unmanaged kept", "default,vip,claude", "claude,foo", "default,vip,claude"},
		{"dedup", "claude,claude", "claude,claude", "claude"},
		{"extra slots dropped", "claude,gemini,x", "new", "new,x"},
		{"unmanaged draft ignored", "x", "y,z", "x"},
	}
	for _, c := range cases {
		got := joinGroups(mergeManaged(splitGroups(c.live), splitGroups(c.draft), managed))
		if got != c.want {
			t.Errorf("%s: mergeManaged(%q,%q) = %q, want %q", c.name, c.live, c.draft, got, c.want)
		}
	}
}

func TestSwapVendorGroup(t *testing.T) {
	managed := func(g string) bool { return g == "claude" || g == "gemini" }
	cases := []struct {
		name       string
		in         string
		oldG, newG string
		want       string
	}{
		{"replace in place", "default,claude,vip", "claude", "gemini", "default,gemini,vip"},
		{"no old group appends", "default,vip", "", "gemini", "default,vip,gemini"},
		{"clear removes only old", "default,claude,vip", "claude", "", "default,vip"},
		{"other managed removed", "gemini,default,claude", "claude", "claude", "default,claude"},
		{"new already present dedups", "claude,default,gemini", "claude", "gemini", "gemini,default"},
		{"no old uses first managed position", "default,gemini,vip", "", "claude", "default,claude,vip"},
	}
	for _, c := range cases {
		got := joinGroups(swapVendorGroup(splitGroups(c.in), c.oldG, c.newG, managed))
		if got != c.want {
			t.Errorf("%s: swap(%q,%q,%q) = %q, want %q", c.name, c.in, c.oldG, c.newG, got, c.want)
		}
	}
}

func TestRenameGroup(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"a,old,b", "a,new,b"},
		{"old,new", "new"},
		{"a,b", "a,b"},
	}
	for _, c := range cases {
		if got := joinGroups(renameGroup(splitGroups(c.in), "old", "new")); got != c.want {
			t.Errorf("renameGroup(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
