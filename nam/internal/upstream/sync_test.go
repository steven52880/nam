package upstream

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/steven52880/nam/internal/crypto"
	"github.com/steven52880/nam/internal/store"
)

type syncEnv struct {
	y   *Syncer
	f   *fakeNewAPI
	srv *httptest.Server
	up  store.Upstream
	box *crypto.Box
}

func newSyncEnv(t *testing.T, n int) *syncEnv {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "nam.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	box, _ := crypto.New("k")
	f := &fakeNewAPI{}
	for i := 1; i <= n; i++ {
		f.tokens = append(f.tokens, map[string]any{"id": i, "name": string(rune('a' + i - 1)), "group": "a", "status": 1, "remain_quota": 500000, "key": "xx**yy"})
	}
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	cred, _ := box.Seal([]byte(`{"access_token":"tok"}`))
	up := store.Upstream{Name: "u", Type: store.UpstreamNewAPI, BaseURL: srv.URL, CredType: store.CredAccessToken, Cred: cred, PlatformUserID: 42, CurrencyRate: 1}
	if err := st.DB.Create(&up).Error; err != nil {
		t.Fatal(err)
	}
	return &syncEnv{y: &Syncer{St: st, Box: box}, f: f, srv: srv, up: up, box: box}
}

func (e *syncEnv) tokens(t *testing.T) map[string]store.UpstreamToken {
	t.Helper()
	var ts []store.UpstreamToken
	if err := e.y.St.DB.Where("upstream_id = ?", e.up.ID).Find(&ts).Error; err != nil {
		t.Fatal(err)
	}
	out := map[string]store.UpstreamToken{}
	for _, tk := range ts {
		out[tk.Name] = tk
	}
	return out
}

func TestKeyHint(t *testing.T) {
	cases := map[string]string{"sk-abcdefghijkl": "sk-ab…ijkl", "short": "sh…", "x": "…"}
	for in, want := range cases {
		if got := keyHint(in); got != want {
			t.Errorf("keyHint(%q)=%q want %q", in, got, want)
		}
	}
}

func TestSyncTokensUpsertAndRetire(t *testing.T) {
	e := newSyncEnv(t, 3)
	ctx := context.Background()
	if n, err := e.y.SyncTokens(ctx, e.up.ID); err != nil || n != 3 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	got := e.tokens(t)
	pt, _ := e.box.Open(got["a"].Key)
	if len(got) != 3 || string(pt) != "sk-full1" || got["a"].KeyHint != "sk…" || got["a"].RemoteID != "1" {
		t.Fatalf("tokens: %+v key=%s", got, pt)
	}
	// manual token survives; token b is used by a channel, token c is not
	manual := store.UpstreamToken{UpstreamID: e.up.ID, Name: "mine", Manual: true, Status: 1}
	e.y.St.DB.Create(&manual)
	b, firstA := got["b"], got["a"].ID
	e.y.St.DB.Create(&store.ChannelExt{ChannelID: 1, TokenID: &b.ID})
	e.f.mu.Lock()
	e.f.tokens = e.f.tokens[:1] // only "a" remains remotely; rename it
	e.f.tokens[0]["name"] = "a2"
	e.f.mu.Unlock()
	if _, err := e.y.SyncTokens(ctx, e.up.ID); err != nil {
		t.Fatal(err)
	}
	got = e.tokens(t)
	if _, ok := got["c"]; ok {
		t.Fatal("unreferenced vanished token must be deleted")
	}
	if got["b"].Status != 0 || got["b"].ID != b.ID {
		t.Fatalf("referenced token must be kept with status 0: %+v", got["b"])
	}
	if _, ok := got["mine"]; !ok {
		t.Fatal("manual token must survive sync")
	}
	if got["a2"].ID != firstA || got["a2"].RemoteID != "1" || len(got) != 3 {
		t.Fatalf("renamed token must be updated in place: %+v", got)
	}
	// key kept when the upstream hides it
	pt, _ = e.box.Open(got["a2"].Key)
	if string(pt) != "sk-full1" {
		t.Fatalf("key lost: %q", pt)
	}
}

func TestSyncGroupsReplacesAndBalance(t *testing.T) {
	e := newSyncEnv(t, 1)
	ctx := context.Background()
	if n, err := e.y.SyncGroups(ctx, e.up.ID); err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	e.y.St.DB.Create(&store.UpstreamGroup{UpstreamID: e.up.ID, Name: "stale", Ratio: 9})
	if _, err := e.y.SyncGroups(ctx, e.up.ID); err != nil {
		t.Fatal(err)
	}
	var gs []store.UpstreamGroup
	var ps []store.UpstreamModelPrice
	e.y.St.DB.Where("upstream_id = ?", e.up.ID).Find(&gs)
	e.y.St.DB.Where("upstream_id = ?", e.up.ID).Find(&ps)
	if len(gs) != 1 || gs[0].Name != "a" || gs[0].Ratio != 0.4 || len(ps) != 2 {
		t.Fatalf("groups=%+v prices=%d", gs, len(ps))
	}
	if err := e.y.SyncBalance(ctx, e.up.ID); err != nil {
		t.Fatal(err)
	}
	u, _ := e.y.St.GetUpstream(ctx, e.up.ID)
	if u.BalanceUSD == nil || *u.BalanceUSD != 5 || u.BalanceAt == 0 || u.LastSyncAt == 0 || u.LastSyncError != "" {
		t.Fatalf("upstream after sync: %+v", u)
	}
}

func TestSyncFailureIsRecorded(t *testing.T) {
	e := newSyncEnv(t, 1)
	e.srv.Close()
	ctx := context.Background()
	for i := 1; i <= 2; i++ {
		if _, err := e.y.SyncTokens(ctx, e.up.ID); err == nil {
			t.Fatal("expected error")
		}
		u, _ := e.y.St.GetUpstream(ctx, e.up.ID)
		if u.ConsecutiveFail != i || u.LastSyncError == "" {
			t.Fatalf("round %d: %+v", i, u)
		}
	}
	msg, failed := e.y.SyncAll(ctx, WhatBalance)
	if failed != 1 || msg == "" {
		t.Fatalf("SyncAll: %q %d", msg, failed)
	}
}

func TestTestReturnsFiveChecks(t *testing.T) {
	e := newSyncEnv(t, 2)
	site, _ := SiteFor(e.up, e.box)
	res := e.y.Test(context.Background(), site)
	if len(res) != 5 {
		t.Fatalf("checks: %+v", res)
	}
	for _, r := range res {
		if !r.OK || r.Skipped {
			t.Errorf("check %s: %+v", r.Name, r)
		}
	}
	// sk-key upstream: pricing is unsupported → skipped, not failed
	sk := Site{Type: store.UpstreamSKKey, BaseURL: e.srv.URL, Cred: Credential{Key: "k"}}
	res = e.y.Test(context.Background(), sk)
	if !res[3].Skipped || res[3].OK {
		t.Fatalf("pricing check on sk-key: %+v", res[3])
	}
}
