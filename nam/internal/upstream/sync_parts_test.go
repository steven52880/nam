package upstream

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/steven52880/nam/internal/store"
)

func (e *syncEnv) parts(t *testing.T) store.SyncParts {
	t.Helper()
	u, err := e.y.St.GetUpstream(context.Background(), e.up.ID)
	if err != nil {
		t.Fatal(err)
	}
	return u.SyncParts
}

func wantPart(t *testing.T, ps store.SyncParts, name, state string, count *int) store.SyncPart {
	t.Helper()
	p, ok := ps[name]
	if !ok {
		t.Fatalf("part %q missing: %+v", name, ps)
	}
	if p.State != state || p.At == 0 {
		t.Fatalf("part %q: %+v, want state %q", name, p, state)
	}
	if (p.Count == nil) != (count == nil) || (count != nil && *p.Count != *count) {
		t.Fatalf("part %q count: %+v, want %v", name, p.Count, count)
	}
	return p
}

func TestSyncPartsSuccess(t *testing.T) {
	e := newSyncEnv(t, 3)
	ctx := context.Background()
	for _, w := range []string{WhatTokens, WhatGroups, WhatBalance} {
		if _, err := e.y.Sync(ctx, e.up.ID, w); err != nil {
			t.Fatal(w, err)
		}
	}
	ps := e.parts(t)
	wantPart(t, ps, "tokens", "ok", intp(3))
	wantPart(t, ps, "groups", "ok", intp(1))
	wantPart(t, ps, "model_prices", "ok", intp(2))
	b := wantPart(t, ps, "balance", "ok", nil)
	if b.Message != "" {
		t.Fatalf("balance message: %q", b.Message)
	}
}

func TestSyncPartsUnsupported(t *testing.T) {
	e := newSyncEnv(t, 1)
	cred, _ := e.box.Seal([]byte(`{"key":"k"}`))
	if err := e.y.St.DB.Model(&store.Upstream{}).Where("id = ?", e.up.ID).
		Updates(map[string]any{"type": store.UpstreamSKKey, "cred_type": store.CredSKKey, "cred": cred}).Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if n, err := e.y.Sync(ctx, e.up.ID, WhatGroups); err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	ps := e.parts(t)
	g := wantPart(t, ps, "groups", "unsupported", nil)
	m := wantPart(t, ps, "model_prices", "unsupported", nil)
	if g.Message != "该上游类型不支持" || m.Message != "该上游类型不支持" {
		t.Fatalf("messages: %+v %+v", g, m)
	}
	u, _ := e.y.St.GetUpstream(ctx, e.up.ID)
	if u.LastSyncError != "" || u.ConsecutiveFail != 0 || u.LastSyncAt == 0 {
		t.Fatalf("unsupported must stay a success for last_sync_*: %+v", u)
	}
}

func TestSyncPartsFailTruncatesMessage(t *testing.T) {
	e := newSyncEnv(t, 1)
	e.srv.Close()
	ctx := context.Background()
	if _, err := e.y.Sync(ctx, e.up.ID, WhatTokens); err == nil {
		t.Fatal("expected error")
	}
	if _, err := e.y.Sync(ctx, e.up.ID, WhatGroups); err == nil {
		t.Fatal("expected error")
	}
	ps := e.parts(t)
	p := wantPart(t, ps, "tokens", "fail", nil)
	if p.Message == "" || len([]rune(p.Message)) > partMsgLimit {
		t.Fatalf("message: %q", p.Message)
	}
	wantPart(t, ps, "groups", "fail", nil)
	wantPart(t, ps, "model_prices", "fail", nil)
	if _, ok := ps["balance"]; ok {
		t.Fatalf("balance was never synced: %+v", ps)
	}
	u, _ := e.y.St.GetUpstream(ctx, e.up.ID)
	if u.ConsecutiveFail != 2 || u.LastSyncError == "" {
		t.Fatalf("last_sync_*: %+v", u)
	}
	long := outcome("", errString(strings.Repeat("长", 1000)), nil)
	if len([]rune(long.Message)) != partMsgLimit {
		t.Fatalf("truncation: %d", len([]rune(long.Message)))
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestSyncPartsGroupsWithoutModelPrices(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/pricing", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "group_ratio": map[string]any{"a": 0.4}, "data": []any{}})
	})
	e := newSyncEnv(t, 1)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	if err := e.y.St.DB.Model(&store.Upstream{}).Where("id = ?", e.up.ID).Update("base_url", srv.URL).Error; err != nil {
		t.Fatal(err)
	}
	if n, err := e.y.Sync(context.Background(), e.up.ID, WhatGroups); err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	ps := e.parts(t)
	wantPart(t, ps, "groups", "ok", intp(1))
	m := wantPart(t, ps, "model_prices", "empty", intp(0))
	if m.Message != "上游没有提供逐模型价格" {
		t.Fatalf("message: %q", m.Message)
	}
}

func TestSyncPartsUpdateIndependently(t *testing.T) {
	e := newSyncEnv(t, 2)
	ctx := context.Background()
	if _, err := e.y.Sync(ctx, e.up.ID, WhatBalance); err != nil {
		t.Fatal(err)
	}
	if _, err := e.y.Sync(ctx, e.up.ID, WhatGroups); err != nil {
		t.Fatal(err)
	}
	ps := e.parts(t)
	wantPart(t, ps, "balance", "ok", nil)
	wantPart(t, ps, "groups", "ok", intp(1))
	wantPart(t, ps, "model_prices", "ok", intp(2))
	if _, ok := ps["tokens"]; ok {
		t.Fatalf("tokens not synced yet: %+v", ps)
	}
	// a later token sync keeps the others untouched
	if _, err := e.y.Sync(ctx, e.up.ID, WhatTokens); err != nil {
		t.Fatal(err)
	}
	ps = e.parts(t)
	if len(ps) != 4 {
		t.Fatalf("want 4 parts: %+v", ps)
	}
	wantPart(t, ps, "tokens", "ok", intp(2))
	wantPart(t, ps, "balance", "ok", nil)
}

func TestSyncPartsLegacyColumnValues(t *testing.T) {
	for name, raw := range map[string]any{"null": nil, "empty": "", "garbage": "not json", "array": "[]"} {
		t.Run(name, func(t *testing.T) {
			e := newSyncEnv(t, 1)
			if err := e.y.St.DB.Exec("UPDATE upstreams SET sync_parts = ? WHERE id = ?", raw, e.up.ID).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := e.y.Sync(context.Background(), e.up.ID, WhatTokens); err != nil {
				t.Fatal(err)
			}
			wantPart(t, e.parts(t), "tokens", "ok", intp(1))
		})
	}
}

func TestSyncPartsConcurrentWritesDoNotClobber(t *testing.T) {
	e := newSyncEnv(t, 1)
	ctx := context.Background()
	var wg sync.WaitGroup
	for _, w := range []string{WhatTokens, WhatGroups, WhatBalance} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = e.y.Sync(ctx, e.up.ID, w)
		}()
	}
	wg.Wait()
	if ps := e.parts(t); len(ps) != 4 {
		t.Fatalf("want 4 parts: %+v", ps)
	}
}

func TestSyncPartsNilMarshalsAsEmptyObject(t *testing.T) {
	b, err := json.Marshal(store.Upstream{})
	if err != nil || !strings.Contains(string(b), `"sync_parts":{}`) {
		t.Fatalf("%s %v", b, err)
	}
}
