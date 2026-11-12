package upstream

import (
	"context"
	"testing"
)

// Token sync must never overwrite the operator-chosen Models list (U10).
func TestSyncTokensKeepsModels(t *testing.T) {
	e := newSyncEnv(t, 2)
	ctx := context.Background()
	if _, err := e.y.SyncTokens(ctx, e.up.ID); err != nil {
		t.Fatal(err)
	}
	a := e.tokens(t)["a"]
	if err := e.y.St.DB.Model(&a).Update("models", `["m1","m2"]`).Error; err != nil {
		t.Fatal(err)
	}
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
	if len(got.Models) != 2 || got.Models[0] != "m1" || got.Models[1] != "m2" {
		t.Fatalf("models lost after sync: %v", got.Models)
	}
}
