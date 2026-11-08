package core_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/steven52880/nam/core"
	"github.com/steven52880/nam/ports/portstest"
)

// fakeSite is a minimal NewAPI-style upstream: 3 tokens a,b,c, one group, a generic balance route.
type fakeSite struct {
	mu     sync.Mutex
	tokens []map[string]any
}

func newFakeSite() *fakeSite {
	f := &fakeSite{}
	for i, n := range []string{"a", "b", "c"} {
		f.tokens = append(f.tokens, map[string]any{"id": i + 1, "name": n, "group": n, "status": 1,
			"remain_quota": 1000000, "unlimited_quota": false, "key": "ab**cd"})
	}
	return f
}

func (f *fakeSite) handler() http.Handler {
	env := func(w http.ResponseWriter, data any) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "", "data": data})
	}
	authed := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "tok" {
				http.Error(w, `{"success":false,"message":"无权进行此操作，未登录"}`, http.StatusUnauthorized)
				return
			}
			h(w, r)
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, _ *http.Request) {
		env(w, map[string]any{"quota_per_unit": 500000, "version": "v-test"})
	})
	mux.HandleFunc("/api/user/self", authed(func(w http.ResponseWriter, _ *http.Request) {
		env(w, map[string]any{"id": 42, "quota": 2500000})
	}))
	mux.HandleFunc("/api/pricing", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true, "group_ratio": map[string]any{"a": 0.4, "b": 0.5},
			"data": []map[string]any{{"model_name": "test-model", "quota_type": 0, "model_ratio": 1.5, "completion_ratio": 5,
				"enable_groups": []string{"a", "b"}}},
		})
	})
	mux.HandleFunc("/api/token/", authed(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		env(w, map[string]any{"items": f.tokens, "total": len(f.tokens), "page": 1})
	}))
	mux.HandleFunc("POST /api/token/{id}/key", authed(func(w http.ResponseWriter, r *http.Request) {
		env(w, map[string]any{"key": "abcdefghijkl-" + r.PathValue("id")})
	}))
	mux.HandleFunc("/_custom/bal", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"left":12345}}`))
	})
	return mux
}

type m2 struct {
	t    *testing.T
	h    http.Handler
	fake *portstest.Fake
	site *httptest.Server
}

func newM2(t *testing.T) *m2 {
	t.Helper()
	fake := portstest.NewFake()
	c, err := core.New(core.Config{DBPath: filepath.Join(t.TempDir(), "nam.db"), Secret: "test-secret"}, fake.Ports())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Stop)
	runCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	c.Start(runCtx) // jobs/consistency?wait=1 needs the scheduler loop
	site := httptest.NewServer(newFakeSite().handler())
	t.Cleanup(site.Close)
	return &m2{t: t, h: c.Handler(), fake: fake, site: site}
}

type reply struct {
	Code int
	Raw  string
	Body struct {
		Success bool            `json:"success"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
}

func (m *m2) call(method, path string, body any) reply {
	m.t.Helper()
	var raw string
	if body != nil {
		b, _ := json.Marshal(body)
		raw = string(b)
	}
	w := do(m.h, method, "/api/nam"+path, raw, "100")
	r := reply{Code: w.Code, Raw: w.Body.String()}
	_ = json.Unmarshal(w.Body.Bytes(), &r.Body)
	return r
}

func (m *m2) must(method, path string, body any) reply {
	m.t.Helper()
	r := m.call(method, path, body)
	if r.Code != 200 {
		m.t.Fatalf("%s %s: %d %s", method, path, r.Code, r.Raw)
	}
	return r
}

func (r reply) into(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.Body.Data, v); err != nil {
		t.Fatalf("decode %s: %v", r.Raw, err)
	}
}

type chRow struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Priority    int64    `json:"priority"`
	Weight      uint     `json:"weight"`
	SyncState   string   `json:"sync_state"`
	DriftFields []string `json:"drift_fields"`
}

func (m *m2) channels() []chRow {
	m.t.Helper()
	var d struct {
		Items      []chRow `json:"items"`
		DraftCount int     `json:"draft_count"`
	}
	m.must("GET", "/channels", nil).into(m.t, &d)
	return d.Items
}

func (m *m2) auditTotal(kind string) int {
	m.t.Helper()
	var d struct {
		Total int `json:"total"`
	}
	m.must("GET", "/audit?kind="+kind+"&limit=1", nil).into(m.t, &d)
	return d.Total
}
