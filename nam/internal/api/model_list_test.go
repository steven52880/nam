package api

import (
	"slices"
	"testing"
)

func TestDedupeSorted(t *testing.T) {
	got := dedupeSorted([]string{"b", " a ", "b", "", "c", "a"})
	if want := []string{"a", "b", "c"}; !slices.Equal(got, want) {
		t.Fatalf("dedupeSorted = %v, want %v", got, want)
	}
	if got := dedupeSorted(nil); got == nil || len(got) != 0 {
		t.Fatalf("nil input must give an empty non-nil slice, got %#v", got)
	}
}
