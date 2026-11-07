package safego

import (
	"strings"
	"sync"
	"testing"
)

func TestGoRecovers(t *testing.T) {
	var got string
	var mu sync.Mutex
	SetPanicHook(func(name string, r any, _ []byte) {
		mu.Lock()
		got = name
		mu.Unlock()
	})
	defer SetPanicHook(nil)
	var wg sync.WaitGroup
	GoWG(&wg, "boom", func() { panic("x") })
	wg.Wait()
	mu.Lock()
	defer mu.Unlock()
	if got != "boom" {
		t.Fatalf("hook not called: %q", got)
	}
}

func TestRunReturnsError(t *testing.T) {
	err := Run("r", func() error { panic("y") })
	if err == nil || !strings.Contains(err.Error(), "panic in r") {
		t.Fatalf("got %v", err)
	}
}
