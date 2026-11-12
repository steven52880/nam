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

func TestOneHubCookieBranch(t *testing.T) {
	var logins atomic.Int32
	var gotCookie, gotUser string
	inner := newOneHubFake(&logins)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/user/self" {
			gotCookie, gotUser = r.Header.Get("Cookie"), r.Header.Get("New-Api-User")
		}
		inner.ServeHTTP(w, r)
	}))
	defer srv.Close()
	s := Site{Type: store.UpstreamOneHub, BaseURL: srv.URL, CredType: store.CredCookie, Cred: Credential{Cookie: "abc"}}
	b, err := oneHubAdapter{}.Balance(context.Background(), s)
	if err != nil || !near(b.USD, 2.5) {
		t.Fatalf("got %+v err=%v", b, err)
	}
	if gotCookie != "session=abc" || gotUser != "" || logins.Load() != 0 {
		t.Fatalf("cookie=%q user=%q logins=%d", gotCookie, gotUser, logins.Load())
	}

	s.Cred.Cookie = "stale-value"
	err = oneHubAdapter{}.Verify(context.Background(), s)
	if !errors.Is(err, ErrAuth) || strings.Contains(err.Error(), "stale-value") {
		t.Fatalf("rejected cookie: %v", err)
	}
	s.Cred.Cookie = ""
	if err := (oneHubAdapter{}).Verify(context.Background(), s); !errors.Is(err, ErrAuth) {
		t.Fatalf("empty cookie: %v", err)
	}
}

func TestOneHubPasswordLoginCaptchaHint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"message":"Turnstile token is empty"}`))
	}))
	defer srv.Close()
	s := Site{Type: store.UpstreamOneHub, BaseURL: srv.URL, CredType: store.CredPassword, Cred: Credential{Username: "u", Password: "p"}}
	err := (oneHubAdapter{}).Verify(context.Background(), s)
	if !errors.Is(err, ErrAuth) || !strings.Contains(err.Error(), "请改用会话 Cookie") {
		t.Fatalf("got %v", err)
	}
}

func TestOneHubProbeLoginCaptcha(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		envelope(w, map[string]any{"quota_per_unit": 500000, "version": "v0.12.0", "PaymentUSDRate": 7, "turnstile_check": true})
	}))
	defer srv.Close()
	r, err := oneHubAdapter{}.Probe(context.Background(), Site{BaseURL: srv.URL})
	if err != nil || !r.LoginCaptcha || !strings.HasSuffix(r.Evidence, "；登录需要人机验证") {
		t.Fatalf("got %+v err=%v", r, err)
	}
}
