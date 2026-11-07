package standalone

import (
	"context"
	"testing"
	"time"

	"github.com/steven52880/nam/ports/portstest"
)

func newTestPorts(t *testing.T, gap time.Duration) (*fakeNewAPI, *Client, portsAlias) {
	t.Helper()
	f := newFakeNewAPI(t)
	cfg := f.config()
	p, c, err := build(cfg, newClient(cfg, gap))
	if err != nil {
		t.Fatal(err)
	}
	return f, c, p
}

func TestContract(t *testing.T) {
	_, _, p := newTestPorts(t, time.Millisecond)
	portstest.RunContract(t, portstest.ContractEnv{Ports: p, SkipTest: false})
}

func TestVerifyRoot(t *testing.T) {
	f, c, _ := newTestPorts(t, time.Millisecond)
	if err := c.VerifyRoot(context.Background()); err != nil {
		t.Fatalf("verify root: %v", err)
	}
	bad := f.config()
	bad.Token = "wrong"
	if err := newClient(bad, time.Millisecond).VerifyRoot(context.Background()); err == nil {
		t.Fatal("wrong token must fail")
	}
}

func TestRequestsRespectMinGap(t *testing.T) {
	f, c, _ := newTestPorts(t, minRequestGap)
	for i := 0; i < 20; i++ {
		if err := c.call(context.Background(), "GET", "/api/status", nil, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.times) != 20 {
		t.Fatalf("server saw %d requests, want 20", len(f.times))
	}
	for i := 1; i < len(f.times); i++ {
		if gap := time.Duration(f.times[i] - f.times[i-1]); gap < minRequestGap-5*time.Millisecond {
			t.Fatalf("gap %d = %v, want >= %v", i, gap, minRequestGap)
		}
	}
}
