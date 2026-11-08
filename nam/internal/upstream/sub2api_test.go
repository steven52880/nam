package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/steven52880/nam/internal/store"
)

const s2JWT = "jwt-secret-value"

func s2Reply(w http.ResponseWriter, data any) {
	_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "message": "ok", "data": data})
}

func newSub2APIFake(logins *atomic.Int32) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/settings/public", func(w http.ResponseWriter, _ *http.Request) {
		s2Reply(w, map[string]any{"version": "1.2.3"})
	})
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]any
		_ = json.NewDecoder(r.Body).Decode(&raw)
		logins.Add(1)
		if raw["email"] != "me@x.com" || raw["password"] != "pw" || raw["not_in_cn_confirmed"] != true {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":401,"message":"invalid credentials"}`))
			return
		}
		s2Reply(w, map[string]any{"access_token": s2JWT, "refresh_token": "r", "expires_in": 3600})
	})
	authed := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer "+s2JWT {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"code":401,"message":"unauthorized"}`))
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("/api/v1/auth/me", authed(func(w http.ResponseWriter, _ *http.Request) {
		s2Reply(w, map[string]any{"id": 9, "balance": 12.5})
	}))
	mux.HandleFunc("/api/v1/keys", authed(func(w http.ResponseWriter, _ *http.Request) {
		s2Reply(w, map[string]any{"total": 3, "items": []map[string]any{
			{"id": 1, "name": "k1", "key": "sk-one", "status": "active", "quota": 10, "quota_used": 4,
				"group": map[string]any{"id": 5, "name": "g5", "description": "five", "rate_multiplier": 1.5}},
			{"id": 2, "name": "k2", "key": "sk-two", "status": "active", "quota": 0,
				"group": map[string]any{"id": 5, "name": "g5", "rate_multiplier": 1.5}},
			{"id": 3, "name": "k3", "key": "sk-three", "status": 2,
				"group": map[string]any{"id": 6, "name": "g6", "rate_multiplier": 2}},
		}})
	}))
	mux.HandleFunc("/api/v1/groups/rates", authed(func(w http.ResponseWriter, _ *http.Request) {
		s2Reply(w, map[string]float64{"5": 0.8})
	}))
	mux.HandleFunc("/api/v1/announcements", authed(func(w http.ResponseWriter, _ *http.Request) {
		s2Reply(w, []map[string]any{{"title": "T1", "content": "body1"}, {"title": "T2", "content": ""}})
	}))
	return mux
}

func s2PasswordSite(url string) Site {
	return Site{Type: store.UpstreamSub2API, BaseURL: url, CredType: store.CredPassword, Cred: Credential{Username: "me@x.com", Password: "pw"}}
}

func s2TokenSite(url, tok string) Site {
	return Site{Type: store.UpstreamSub2API, BaseURL: url, CredType: store.CredAccessToken, Cred: Credential{AccessToken: tok}}
}

func TestSub2APIProbe(t *testing.T) {
	var logins atomic.Int32
	srv := httptest.NewServer(newSub2APIFake(&logins))
	defer srv.Close()
	got, err := sub2apiAdapter{}.Probe(context.Background(), s2TokenSite(srv.URL, ""))
	if err != nil || got.Type != store.UpstreamSub2API || got.Version != "1.2.3" {
		t.Fatalf("got %+v err=%v", got, err)
	}
	// A non-sub2api JSON body (no {code}) must not be accepted.
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"data":{}}`))
	}))
	defer other.Close()
	if _, err := (sub2apiAdapter{}).Probe(context.Background(), s2TokenSite(other.URL, "")); err == nil {
		t.Fatal("non-sub2api envelope must fail probe")
	}
}

func TestSub2APIVerifyAuthRejected(t *testing.T) {
	var logins atomic.Int32
	srv := httptest.NewServer(newSub2APIFake(&logins))
	defer srv.Close()
	if err := (sub2apiAdapter{}).Verify(context.Background(), s2TokenSite(srv.URL, s2JWT)); err != nil {
		t.Fatal(err)
	}
	err := (sub2apiAdapter{}).Verify(context.Background(), s2TokenSite(srv.URL, "bad-token-xyz"))
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("want ErrAuth, got %v", err)
	}
	if strings.Contains(err.Error(), "bad-token-xyz") {
		t.Fatal("credential leaked into error")
	}
}

func TestSub2APIPasswordLoginOncePerCall(t *testing.T) {
	var logins atomic.Int32
	srv := httptest.NewServer(newSub2APIFake(&logins))
	defer srv.Close()
	b, err := sub2apiAdapter{}.Balance(context.Background(), s2PasswordSite(srv.URL))
	if err != nil || !near(b.USD, 12.5) {
		t.Fatalf("got %+v err=%v", b, err)
	}
	if logins.Load() != 1 {
		t.Fatalf("logins=%d, want 1", logins.Load())
	}
	bad := s2PasswordSite(srv.URL)
	bad.Cred.Password = "nope-password"
	err = (sub2apiAdapter{}).Verify(context.Background(), bad)
	if !errors.Is(err, ErrAuth) || strings.Contains(err.Error(), "nope-password") {
		t.Fatalf("want ErrAuth without secret, got %v", err)
	}
}

func TestSub2APIListTokens(t *testing.T) {
	var logins atomic.Int32
	srv := httptest.NewServer(newSub2APIFake(&logins))
	defer srv.Close()
	ts, err := sub2apiAdapter{}.ListTokens(context.Background(), s2TokenSite(srv.URL, s2JWT))
	if err != nil || len(ts) != 3 {
		t.Fatalf("got %+v err=%v", ts, err)
	}
	if ts[0].Key != "sk-one" || ts[0].Group != "g5" || ts[0].Status != 1 || ts[0].QuotaUSD == nil || !near(*ts[0].QuotaUSD, 6) {
		t.Fatalf("bad token 0: %+v", ts[0])
	}
	if !ts[1].Unlimited || ts[2].Status != 2 {
		t.Fatalf("bad tokens: %+v %+v", ts[1], ts[2])
	}
	if _, err := (sub2apiAdapter{}).CreateToken(context.Background(), s2TokenSite(srv.URL, s2JWT), TokenCreate{}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("want ErrUnsupported, got %v", err)
	}
}

func TestSub2APIPricingGroupRates(t *testing.T) {
	var logins atomic.Int32
	srv := httptest.NewServer(newSub2APIFake(&logins))
	defer srv.Close()
	p, err := sub2apiAdapter{}.Pricing(context.Background(), s2PasswordSite(srv.URL))
	if err != nil || len(p.Groups) != 2 || len(p.Models) != 0 {
		t.Fatalf("got %+v err=%v", p, err)
	}
	// g5 default rate 1.5 is overridden by the per-user rate 0.8; g6 keeps its default 2.
	if p.Groups[0].Name != "g5" || !near(p.Groups[0].Ratio, 0.8) || p.Groups[0].Desc != "five" || !near(p.Groups[1].Ratio, 2) {
		t.Fatalf("bad groups %+v", p.Groups)
	}
}

func TestSub2APIAnnouncements(t *testing.T) {
	var logins atomic.Int32
	srv := httptest.NewServer(newSub2APIFake(&logins))
	defer srv.Close()
	a, err := sub2apiAdapter{}.Announcements(context.Background(), s2TokenSite(srv.URL, s2JWT))
	if err != nil || a.Content != "T1\nbody1\n\nT2" {
		t.Fatalf("got %q err=%v", a.Content, err)
	}
}
