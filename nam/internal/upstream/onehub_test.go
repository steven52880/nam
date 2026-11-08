package upstream

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/steven52880/nam/internal/store"
)

func newOneHubFake(logins *atomic.Int32) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, _ *http.Request) {
		envelope(w, map[string]any{"quota_per_unit": 500000, "version": "v0.12.0", "PaymentUSDRate": 7, "UptimeEnabled": false})
	})
	mux.HandleFunc("/api/user/login", func(w http.ResponseWriter, _ *http.Request) {
		logins.Add(1)
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc", Path: "/"})
		envelope(w, map[string]any{"id": 7, "username": "u"})
	})
	authed := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie("session")
			if r.Header.Get("Authorization") != "tok" && (err != nil || c.Value != "abc") {
				_, _ = w.Write([]byte(`{"success":false,"message":"无权进行此操作，access token 无效"}`))
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("/api/user/self", authed(func(w http.ResponseWriter, _ *http.Request) {
		envelope(w, map[string]any{"id": 7, "quota": 1250000, "used_quota": 5})
	}))
	mux.HandleFunc("/api/token/", authed(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			envelope(w, nil)
			return
		}
		envelope(w, map[string]any{"page": 1, "size": 100, "total_count": 2, "data": []map[string]any{
			{"id": 2, "name": "masked", "key": "sk-ab**cd", "status": 1, "remain_quota": 0, "unlimited_quota": true, "group": "a"},
			{"id": 1, "name": "main", "key": "abc123", "status": 1, "remain_quota": 250000, "unlimited_quota": false, "group": "a"},
		}})
	}))
	mux.HandleFunc("/api/notice", func(w http.ResponseWriter, _ *http.Request) { envelope(w, " 公告 ") })
	mux.HandleFunc("/api/user_group_map", func(w http.ResponseWriter, _ *http.Request) {
		envelope(w, map[string]any{
			"a": map[string]any{"symbol": "a", "name": "A组", "ratio": 0.4, "public": true},
			"b": map[string]any{"symbol": "b", "name": "B组", "ratio": 1, "public": true},
		})
	})
	mux.HandleFunc("/api/available_model", func(w http.ResponseWriter, _ *http.Request) {
		envelope(w, map[string]any{
			"m1": map[string]any{"groups": []string{"a", "zz"}, "owned_by": "openai",
				"price": map[string]any{"type": "tokens", "input": 15, "output": 30, "extra_ratios": map[string]float64{"cached_read_tokens": 0.5}}},
			"m2": map[string]any{"groups": []string{"b"}, "owned_by": "x",
				"price": map[string]any{"type": "times", "input": 25, "output": 0}},
		})
	})
	return mux
}

func oneHubSite(url string) Site {
	return Site{Type: store.UpstreamOneHub, BaseURL: url, CredType: store.CredAccessToken, Cred: Credential{AccessToken: "tok"}}
}

func TestOneHubProbeNeedsOneHubMarkers(t *testing.T) {
	var logins atomic.Int32
	srv := httptest.NewServer(newOneHubFake(&logins))
	defer srv.Close()
	got, err := oneHubAdapter{}.Probe(context.Background(), oneHubSite(srv.URL))
	if err != nil || got.Type != store.UpstreamOneHub || got.Version != "v0.12.0" {
		t.Fatalf("got %+v err=%v", got, err)
	}
	// A plain NewAPI /api/status must not be detected as OneHub.
	na := httptest.NewServer((&fakeNewAPI{}).handler())
	defer na.Close()
	if _, err := (oneHubAdapter{}).Probe(context.Background(), oneHubSite(na.URL)); err == nil {
		t.Fatal("NewAPI status must not probe as onehub")
	}
}

func TestOneHubVerifyAuthRejected(t *testing.T) {
	var logins atomic.Int32
	srv := httptest.NewServer(newOneHubFake(&logins))
	defer srv.Close()
	s := oneHubSite(srv.URL)
	if err := (oneHubAdapter{}).Verify(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	s.Cred.AccessToken = "wrong-secret"
	err := (oneHubAdapter{}).Verify(context.Background(), s)
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("want ErrAuth, got %v", err)
	}
	if strings.Contains(err.Error(), "wrong-secret") {
		t.Fatal("credential leaked into error")
	}
}

func TestOneHubBalanceAndPasswordLogin(t *testing.T) {
	var logins atomic.Int32
	srv := httptest.NewServer(newOneHubFake(&logins))
	defer srv.Close()
	s := Site{Type: store.UpstreamOneHub, BaseURL: srv.URL, CredType: store.CredPassword, Cred: Credential{Username: "u", Password: "p"}}
	b, err := oneHubAdapter{}.Balance(context.Background(), s)
	if err != nil || !near(b.USD, 2.5) {
		t.Fatalf("got %+v err=%v", b, err)
	}
	if logins.Load() != 1 {
		t.Fatalf("logins=%d, want 1 per call", logins.Load())
	}
}

func TestOneHubListTokens(t *testing.T) {
	var logins atomic.Int32
	srv := httptest.NewServer(newOneHubFake(&logins))
	defer srv.Close()
	ts, err := oneHubAdapter{}.ListTokens(context.Background(), oneHubSite(srv.URL))
	if err != nil || len(ts) != 2 {
		t.Fatalf("got %+v err=%v", ts, err)
	}
	if ts[0].Key != "" || !ts[0].Unlimited {
		t.Fatalf("masked key must be blanked: %+v", ts[0])
	}
	if ts[1].Key != "sk-abc123" || ts[1].QuotaUSD == nil || !near(*ts[1].QuotaUSD, 0.5) || ts[1].Group != "a" {
		t.Fatalf("bad token %+v", ts[1])
	}
	created, err := oneHubAdapter{}.CreateToken(context.Background(), oneHubSite(srv.URL), TokenCreate{Name: "main", Group: "a"})
	if err != nil || created.RemoteID != "1" {
		t.Fatalf("got %+v err=%v", created, err)
	}
}

func TestOneHubPricingConversion(t *testing.T) {
	var logins atomic.Int32
	srv := httptest.NewServer(newOneHubFake(&logins))
	defer srv.Close()
	p, err := oneHubAdapter{}.Pricing(context.Background(), oneHubSite(srv.URL))
	if err != nil || len(p.Groups) != 2 || len(p.Models) != 2 {
		t.Fatalf("got %+v err=%v", p, err)
	}
	m1, m2 := p.Models[0], p.Models[1]
	if m1.Model != "m1" || m1.Group != "a" {
		t.Fatalf("m1 (group zz has no ratio and must be skipped): %+v", m1)
	}
	// input 15 * group 0.4 * 1e6/5e5 = 12; output 30 → 24; cache read 0.5 override; write default 1.25.
	if !near(*m1.Input, 12) || !near(*m1.Output, 24) || !near(*m1.CacheRead, 6) || !near(*m1.CacheWrite, 15) {
		t.Fatalf("bad token price %+v", m1)
	}
	// times: 25 * 1.0 * 1000 / 5e5 = 0.05 USD per request.
	if m2.Model != "m2" || m2.PerRequest == nil || !near(*m2.PerRequest, 0.05) || m2.Input != nil {
		t.Fatalf("bad per-request price %+v", m2)
	}
}

func TestOneHubAnnouncements(t *testing.T) {
	var logins atomic.Int32
	srv := httptest.NewServer(newOneHubFake(&logins))
	defer srv.Close()
	a, err := oneHubAdapter{}.Announcements(context.Background(), oneHubSite(srv.URL))
	if err != nil || a.Content != "公告" {
		t.Fatalf("got %+v err=%v", a, err)
	}
}
