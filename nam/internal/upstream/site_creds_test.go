package upstream

import (
	"context"
	"testing"
	"time"
)

func TestDBCredStoreRoundTrip(t *testing.T) {
	e := newSyncEnv(t, 0)
	ctx := context.Background()
	cs := NewCredStore(e.y.St, e.box, e.up.ID)

	got, err := cs.Load(ctx)
	if err != nil || got.AccessToken != "tok" {
		t.Fatalf("initial load: %+v err=%v", got, err)
	}
	before := time.Now().Unix()
	next := Credential{AccessToken: "a2", RefreshToken: "r2", ExpiresAt: 1900000000}
	if err := cs.Save(ctx, next); err != nil {
		t.Fatal(err)
	}
	got, err = cs.Load(ctx)
	if err != nil || got != next {
		t.Fatalf("reload: %+v err=%v", got, err)
	}
	u, err := e.y.St.GetUpstream(ctx, e.up.ID)
	if err != nil {
		t.Fatal(err)
	}
	if u.CredUpdatedAt < before {
		t.Fatalf("cred_updated_at=%d, want >= %d", u.CredUpdatedAt, before)
	}
	// SiteFor must see the rotated pair too.
	site, err := SiteFor(u, e.box)
	if err != nil || site.Cred != next {
		t.Fatalf("SiteFor: %+v err=%v", site.Cred, err)
	}
}
