package upstream

import (
	"context"
	"testing"
)

// Token sync must update a remote token in place (same row id) instead of replace it.
func TestSyncTokensUpdatesInPlace(t *testing.T) {
	e := newSyncEnv(t, 2)
	ctx := context.Background()
	if _, err := e.y.SyncTokens(ctx, e.up.ID); err != nil {
		t.Fatal(err)
	}
	a := e.tokens(t)["a"]
	e.f.mu.Lock()
	e.f.tokens[0]["group"] = "changed" // force a real update of the row
	e.f.mu.Unlock()
	if _, err := e.y.SyncTokens(ctx, e.up.ID); err != nil {
		t.Fatal(err)
	}
	got := e.tokens(t)["a"]
	if got.ID != a.ID || got.Group != "changed" {
		t.Fatalf("token not updated in place: %+v", got)
	}
}
