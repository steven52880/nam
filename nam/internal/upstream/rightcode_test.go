package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/steven52880/nam/internal/store"
)

const (
	rcTok   = "00000000-0000-0000-0000-000000000000"
	rcPast  = "2000-01-01T00:00:00"
	rcLater = "2999-01-01T00:00:00"
)

type rcFake struct {
	mu            sync.Mutex
	balance       any
	banned        any
	createBody    map[string]any
	createRespID  int
	modelsStatus  int
	effectiveFail bool
	notice        *string
	auths         []string
}

func rcJSON(w http.ResponseWriter, v any) { _ = json.NewEncoder(w).Encode(v) }

var rcKeysFixture = []map[string]any{
	{"id": 1, "bound_upstream_id": 27, "key": "sk-test-1", "name": "bound", "quota_limit": nil, "used_quota": 0,
		"expired_at": nil, "is_active": true, "disabled_by_security_review": nil},
	{"id": 2, "bound_upstream_id": nil, "key": "sk-test-2", "name": "legacy", "quota_limit": nil, "used_quota": 0,
		"expired_at": rcLater, "is_active": true},
	{"id": 3, "bound_upstream_id": 28, "key": "sk-test-3", "name": "limited", "quota_limit": 10, "used_quota": 3.5,
		"is_active": true},
	{"id": 4, "bound_upstream_id": 27, "key": "sk-test-4", "name": "expired", "quota_limit": "5", "used_quota": 9,
		"expired_at": rcPast, "is_active": true},
	{"id": 5, "bound_upstream_id": 99, "key": "sk-test-5", "name": "secrev", "quota_limit": nil, "used_quota": 0,
		"is_active": true, "disabled_by_security_review": true},
}

var rcUpstreamsFixture = map[string]any{"upstreams": []map[string]any{
	{"id": 27, "name": "Codex", "prefix": "/codex", "rate": 0.4},
	{"id": 28, "name": "Claude", "prefix": "/claude", "rate": "2.00000000"},
}}

func (f *rcFake) handler() http.Handler {
	mux := http.NewServeMux()
	authed := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			f.auths = append(f.auths, r.Header.Get("Authorization"))
			f.mu.Unlock()
			if r.Header.Get("Authorization") != "Bearer "+rcTok {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"status":401,"message":"Unauthorized"}`))
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("/upstreams/public", func(w http.ResponseWriter, _ *http.Request) { rcJSON(w, rcUpstreamsFixture) })
	mux.HandleFunc("/upstreams/effective", authed(func(w http.ResponseWriter, _ *http.Request) {
		if f.effectiveFail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		rcJSON(w, rcUpstreamsFixture)
	}))
	mux.HandleFunc("/auth/me", authed(func(w http.ResponseWriter, _ *http.Request) {
		rcJSON(w, map[string]any{"id": 7, "username": "u", "balance": f.balance, "is_banned": f.banned})
	}))
	mux.HandleFunc("/api-key/list", authed(func(w http.ResponseWriter, _ *http.Request) {
		rcJSON(w, map[string]any{"keys": rcKeysFixture, "total": len(rcKeysFixture)})
	}))
	mux.HandleFunc("/api-key/create", authed(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := readAllString(r)
		f.mu.Lock()
		f.createBody = map[string]any{}
		_ = json.Unmarshal([]byte(raw), &f.createBody)
		f.mu.Unlock()
		f.createBody["__raw"] = raw
		rcJSON(w, map[string]any{"id": f.createRespID})
	}))
	mux.HandleFunc("/api-key/1", authed(func(w http.ResponseWriter, _ *http.Request) { rcJSON(w, rcKeysFixture[0]) }))
	mux.HandleFunc("/configs", func(w http.ResponseWriter, _ *http.Request) {
		cfg := map[string]string{"public.other": "x"}
		if f.notice != nil {
			cfg["public.notice"] = *f.notice
		}
		rcJSON(w, cfg)
	})
	mux.HandleFunc("/models/effective", authed(func(w http.ResponseWriter, _ *http.Request) {
		if f.modelsStatus != 0 {
			w.WriteHeader(f.modelsStatus)
			return
		}
		rcJSON(w, rcModelsFixture(false))
	}))
	mux.HandleFunc("/models/public", func(w http.ResponseWriter, _ *http.Request) { rcJSON(w, rcModelsFixture(true)) })
	return mux
}

func readAllString(r *http.Request) (string, error) {
	var sb strings.Builder
	buf := make([]byte, 512)
	for {
		n, err := r.Body.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			return sb.String(), nil
		}
	}
}

func rcModelsFixture(public bool) map[string]any {
	src := "effective"
	if public {
		src = "public"
	}
	return map[string]any{"upstreams": []map[string]any{
		{"name": "Codex", "prefix": "/codex", "models": []map[string]any{
			{"name": "plain-" + src, "is_available": true, "input_price": 5, "output_price": 30, "cache_read_input_price": 0.5,
				"cache_creation_input_price": 0, "request_price": nil, "effective_total_multiplier": "0.4",
				"effective_price_config": map[string]any{"input_price": "2", "output_price": "12", "cache_read_input_price": "0.2", "cache_creation_input_price": "0"}},
			{"name": "tiered", "is_available": true, "effective_total_multiplier": "1.5",
				"effective_price_config": map[string]any{"tiers": []map[string]any{
					{"input_price": "3", "output_price": "18", "cache_read_input_price": "0.3", "cache_creation_input_price": "0", "max_context_length": 200},
					{"input_price": "6", "output_price": "27", "max_context_length": nil}}}},
			{"name": "per-request", "is_available": true, "request_price": 0.25, "effective_total_multiplier": "1",
				"effective_price_config": map[string]any{"request_price": "0.25"}},
			{"name": "empty-cfg", "is_available": false, "input_price": 4, "output_price": 20, "request_price": nil,
				"effective_total_multiplier": "0.5", "effective_price_config": map[string]any{}},
			{"name": "raw-request", "is_available": false, "request_price": 0.1, "effective_total_multiplier": "2",
				"effective_price_config": map[string]any{}},
		}},
	}}
}

func rcSite(url, tok string) Site {
	return Site{Type: store.UpstreamRightCode, BaseURL: url, CredType: store.CredAccessToken, Cred: Credential{AccessToken: tok}}
}

func rcStart(f *rcFake) (*httptest.Server, Site) {
	srv := httptest.NewServer(f.handler())
	return srv, rcSite(srv.URL, rcTok)
}

func TestRightCodeProbe(t *testing.T) {
	srv, s := rcStart(&rcFake{})
	defer srv.Close()
	got, err := rightCodeAdapter{}.Probe(context.Background(), s)
	if err != nil || got.Type != store.UpstreamRightCode || !strings.Contains(got.Evidence, "2 个渠道") {
		t.Fatalf("got %+v err=%v", got, err)
	}
	notFound := httptest.NewServer(http.NotFoundHandler())
	defer notFound.Close()
	if _, err := (rightCodeAdapter{}).Probe(context.Background(), rcSite(notFound.URL, "")); err == nil {
		t.Fatal("404 must fail probe")
	}
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"upstreams":[]}`)) }))
	defer empty.Close()
	if _, err := (rightCodeAdapter{}).Probe(context.Background(), rcSite(empty.URL, "")); err == nil {
		t.Fatal("empty list must fail probe")
	}
}

func TestRightCodeAuth(t *testing.T) {
	f := &rcFake{balance: 1.5}
	srv, s := rcStart(f)
	defer srv.Close()
	ctx := context.Background()
	if err := (rightCodeAdapter{}).Verify(ctx, s); err != nil {
		t.Fatal(err)
	}
	if f.auths[0] != "Bearer "+rcTok {
		t.Fatalf("bad header %q", f.auths[0])
	}
	f.auths = nil
	if err := (rightCodeAdapter{}).Verify(ctx, rcSite(srv.URL, "Bearer "+rcTok)); err != nil {
		t.Fatalf("Bearer-prefixed credential: %v", err)
	}
	if f.auths[0] != "Bearer "+rcTok {
		t.Fatalf("Bearer duplicated: %q", f.auths[0])
	}
	for _, empty := range []string{"", "   ", "Bearer "} {
		if err := (rightCodeAdapter{}).Verify(ctx, rcSite(srv.URL, empty)); !errors.Is(err, ErrAuth) {
			t.Fatalf("empty %q: want ErrAuth, got %v", empty, err)
		}
	}
	err := (rightCodeAdapter{}).Verify(ctx, rcSite(srv.URL, "wrong-token-xyz"))
	if !errors.Is(err, ErrAuth) || strings.Contains(err.Error(), "wrong-token-xyz") {
		t.Fatalf("want ErrAuth without secret, got %v", err)
	}
	pw := s
	pw.CredType = store.CredPassword
	if err := (rightCodeAdapter{}).Verify(ctx, pw); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("password cred: want ErrUnsupported, got %v", err)
	}
	f.banned = true
	if err := (rightCodeAdapter{}).Verify(ctx, s); !errors.Is(err, ErrAuth) || !strings.Contains(err.Error(), "封禁") {
		t.Fatalf("banned: want ErrAuth, got %v", err)
	}
}

func TestRightCodeBalance(t *testing.T) {
	for name, bal := range map[string]any{"number": 497.478201, "string": "497.478201"} {
		srv, s := rcStart(&rcFake{balance: bal})
		b, err := rightCodeAdapter{}.Balance(context.Background(), s)
		srv.Close()
		if err != nil || !near(b.USD, 497.478201) || b.Raw != "balance=497.478201" {
			t.Fatalf("%s: got %+v err=%v", name, b, err)
		}
	}
	srv, s := rcStart(&rcFake{})
	defer srv.Close()
	if _, err := (rightCodeAdapter{}).Balance(context.Background(), s); err == nil {
		t.Fatal("missing balance must fail")
	}
}

func TestRightCodeListTokens(t *testing.T) {
	srv, s := rcStart(&rcFake{})
	defer srv.Close()
	ts, err := rightCodeAdapter{}.ListTokens(context.Background(), s)
	if err != nil || len(ts) != 5 {
		t.Fatalf("got %+v err=%v", ts, err)
	}
	if ts[0].Key != "sk-test-1" || ts[0].RemoteID != "1" || ts[0].Group != "/codex" || ts[0].Status != 1 || !ts[0].Unlimited || ts[0].QuotaUSD != nil {
		t.Fatalf("bound: %+v", ts[0])
	}
	if ts[1].Group != "" || ts[1].Status != 1 || !ts[1].Unlimited {
		t.Fatalf("legacy: %+v", ts[1])
	}
	if ts[2].Group != "/claude" || ts[2].Unlimited || ts[2].QuotaUSD == nil || !near(*ts[2].QuotaUSD, 6.5) {
		t.Fatalf("limited: %+v", ts[2])
	}
	if ts[3].Status != 2 || ts[3].Unlimited || ts[3].QuotaUSD == nil || !near(*ts[3].QuotaUSD, 0) {
		t.Fatalf("expired: %+v", ts[3])
	}
	if ts[4].Status != 2 || ts[4].Group != "#99" || !ts[4].Unlimited {
		t.Fatalf("security review: %+v", ts[4])
	}
}

func TestRightCodeKeyExpiryIsCST(t *testing.T) {
	exp := "2026-01-01T08:00:00" // 00:00 UTC
	k := rcKey{ExpiredAt: &exp}
	if k.expired(time.Date(2025, 12, 31, 23, 59, 0, 0, time.UTC)) {
		t.Fatal("not yet expired")
	}
	if !k.expired(time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC)) {
		t.Fatal("should be expired")
	}
}

func TestRightCodeCreateToken(t *testing.T) {
	f := &rcFake{createRespID: 1}
	srv, s := rcStart(f)
	defer srv.Close()
	ctx := context.Background()
	tok, err := rightCodeAdapter{}.CreateToken(ctx, s, TokenCreate{Name: "nam-x", Group: "/codex"})
	if err != nil || tok.Key != "sk-test-1" || tok.RemoteID != "1" {
		t.Fatalf("got %+v err=%v", tok, err)
	}
	body := f.createBody
	if body["name"] != "nam-x" || body["bound_upstream_id"] != float64(27) || body["allow_wallet"] != true {
		t.Fatalf("bad body %v", body)
	}
	if v, ok := body["allowed_models"].([]any); !ok || len(v) != 0 {
		t.Fatalf("allowed_models %v", body["allowed_models"])
	}
	if v, ok := body["allowed_item_ids"].([]any); !ok || len(v) != 0 {
		t.Fatalf("allowed_item_ids %v", body["allowed_item_ids"])
	}
	if !strings.Contains(body["__raw"].(string), `"quota_limit":null`) {
		t.Fatalf("quota_limit must be JSON null: %v", body["__raw"])
	}
	q := 12.5
	if _, err := (rightCodeAdapter{}).CreateToken(ctx, s, TokenCreate{Name: "q", Group: "/claude", QuotaUSD: &q}); err != nil {
		t.Fatal(err)
	}
	if f.createBody["quota_limit"] != 12.5 || f.createBody["bound_upstream_id"] != float64(28) {
		t.Fatalf("bad body %v", f.createBody)
	}
	for _, g := range []string{"", "/nope"} {
		if _, err := (rightCodeAdapter{}).CreateToken(ctx, s, TokenCreate{Name: "x", Group: g}); err == nil || !strings.Contains(err.Error(), "未知分组") {
			t.Fatalf("group %q: want unknown group error, got %v", g, err)
		}
	}
}

func TestRightCodeCreateTokenFallbackToList(t *testing.T) {
	f := &rcFake{createRespID: 0}
	srv, s := rcStart(f)
	defer srv.Close()
	tok, err := rightCodeAdapter{}.CreateToken(context.Background(), s, TokenCreate{Name: "limited", Group: "/claude"})
	if err != nil || tok.RemoteID != "3" {
		t.Fatalf("got %+v err=%v", tok, err)
	}
	if _, err := (rightCodeAdapter{}).CreateToken(context.Background(), s, TokenCreate{Name: "absent", Group: "/claude"}); err == nil {
		t.Fatal("missing key must error")
	}
}

func rcFind(t *testing.T, ms []ModelPrice, name string) ModelPrice {
	t.Helper()
	for _, m := range ms {
		if m.Model == name {
			return m
		}
	}
	t.Fatalf("model %q not found in %+v", name, ms)
	return ModelPrice{}
}

func rcWant(t *testing.T, label string, got *float64, want float64) {
	t.Helper()
	if got == nil || !near(*got, want) {
		t.Fatalf("%s: got %v want %v", label, got, want)
	}
}

func TestRightCodePricing(t *testing.T) {
	srv, s := rcStart(&rcFake{})
	defer srv.Close()
	p, err := rightCodeAdapter{}.Pricing(context.Background(), s)
	if err != nil || len(p.Groups) != 2 || len(p.Models) != 5 {
		t.Fatalf("got %+v err=%v", p, err)
	}
	if p.Groups[0].Name != "/claude" || !near(p.Groups[0].Ratio, 2) || p.Groups[0].Desc != "Claude" || !near(p.Groups[1].Ratio, 0.4) {
		t.Fatalf("groups %+v", p.Groups)
	}
	m := rcFind(t, p.Models, "plain-effective")
	if m.Group != "/codex" || m.ModelRatio != nil {
		t.Fatalf("plain: %+v", m)
	}
	rcWant(t, "in", m.Input, 2)
	rcWant(t, "out", m.Output, 12)
	rcWant(t, "cr", m.CacheRead, 0.2)
	rcWant(t, "cw", m.CacheWrite, 0)
	tr := rcFind(t, p.Models, "tiered")
	rcWant(t, "tier in", tr.Input, 3)
	rcWant(t, "tier out", tr.Output, 18)
	rcWant(t, "tier cr", tr.CacheRead, 0.3)
	pr := rcFind(t, p.Models, "per-request")
	rcWant(t, "req", pr.PerRequest, 0.25)
	if pr.Input != nil {
		t.Fatalf("per-request should have no input: %+v", pr)
	}
	em := rcFind(t, p.Models, "empty-cfg") // unavailable model is kept; raw × multiplier
	rcWant(t, "fallback in", em.Input, 2)
	rcWant(t, "fallback out", em.Output, 10)
	rr := rcFind(t, p.Models, "raw-request")
	rcWant(t, "raw req", rr.PerRequest, 0.2)
}

func TestRightCodePricingFallsBackToPublic(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusNotFound} {
		srv, s := rcStart(&rcFake{modelsStatus: status})
		p, err := rightCodeAdapter{}.Pricing(context.Background(), s)
		srv.Close()
		if err != nil || len(p.Models) != 5 {
			t.Fatalf("status %d: %+v err=%v", status, p, err)
		}
		rcFind(t, p.Models, "plain-public")
	}
}

func TestRightCodePricingUpstreamsFallback(t *testing.T) {
	srv, s := rcStart(&rcFake{effectiveFail: true})
	defer srv.Close()
	p, err := rightCodeAdapter{}.Pricing(context.Background(), s)
	if err != nil || len(p.Groups) != 2 {
		t.Fatalf("got %+v err=%v", p, err)
	}
	if _, err := (rightCodeAdapter{}).Pricing(context.Background(), rcSite(srv.URL, "bad")); !errors.Is(err, ErrAuth) {
		t.Fatalf("want ErrAuth, got %v", err)
	}
}

func TestRightCodeAnnouncements(t *testing.T) {
	notice := `{"admin":[{"id":"a","description":"<p>one</p>"},{"id":"b","description":"  "},{"id":"c","description":"two"}]}`
	empty := ""
	cases := map[string]struct {
		notice *string
		want   string
	}{
		"normal":  {&notice, "<p>one</p>\n\ntwo"},
		"missing": {nil, ""},
		"empty":   {&empty, ""},
	}
	for name, c := range cases {
		srv, s := rcStart(&rcFake{notice: c.notice})
		a, err := rightCodeAdapter{}.Announcements(context.Background(), s)
		srv.Close()
		if err != nil || a.Content != c.want {
			t.Fatalf("%s: got %q err=%v", name, a.Content, err)
		}
	}
}

func TestRightCodeListModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer sk-test" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"object":"list","data":[{"id":"b","object":"model"},{"id":"a","object":"model"}]}`))
	}))
	defer srv.Close()
	got, err := rightCodeAdapter{}.ListModels(context.Background(), rcSite(srv.URL, rcTok), "sk-test")
	if err != nil || len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got %v err=%v", got, err)
	}
}
