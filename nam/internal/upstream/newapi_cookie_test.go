package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/steven52880/nam/internal/store"
)

func newAPICookieSite(url string, cookie string, uid int) Site {
	return Site{Type: store.UpstreamNewAPI, BaseURL: url, CredType: store.CredCookie, Cred: Credential{Cookie: cookie}, UserID: uid}
}

func TestNewAPICookieSendsHeaders(t *testing.T) {
	f := &fakeNewAPI{}
	var gotCookie string
	inner := f.handler()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/user/self" {
			gotCookie = r.Header.Get("Cookie")
		}
		inner.ServeHTTP(w, r)
	}))
	defer srv.Close()
	b, err := newAPIAdapter{}.Balance(context.Background(), newAPICookieSite(srv.URL, "abc", 42))
	if err != nil || b.USD != 5.0 {
		t.Fatalf("got %+v err=%v", b, err)
	}
	if gotCookie != "session=abc" || f.lastUser != "42" || f.lastAuth != "" || f.logins != 0 {
		t.Fatalf("cookie=%q user=%q auth=%q logins=%d", gotCookie, f.lastUser, f.lastAuth, f.logins)
	}
}

func TestNewAPICookieNeedsUserIDAndCookie(t *testing.T) {
	srv := httptest.NewServer((&fakeNewAPI{}).handler())
	defer srv.Close()
	for name, s := range map[string]Site{
		"no user id": newAPICookieSite(srv.URL, "abc", 0),
		"no cookie":  newAPICookieSite(srv.URL, "", 42),
	} {
		if err := (newAPIAdapter{}).Verify(context.Background(), s); !errors.Is(err, ErrAuth) {
			t.Errorf("%s: want ErrAuth, got %v", name, err)
		}
	}
}

func TestNewAPICookieRejected(t *testing.T) {
	srv := httptest.NewServer((&fakeNewAPI{}).handler())
	defer srv.Close()
	err := (newAPIAdapter{}).Verify(context.Background(), newAPICookieSite(srv.URL, "stale-value", 42))
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("want ErrAuth, got %v", err)
	}
	if strings.Contains(err.Error(), "stale-value") {
		t.Fatal("cookie leaked into error")
	}
}

func TestNewAPIPasswordLoginCaptchaHint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"message":"Turnstile token is empty"}`))
	}))
	defer srv.Close()
	s := Site{BaseURL: srv.URL, CredType: store.CredPassword, Cred: Credential{Username: "u", Password: "p"}}
	err := (newAPIAdapter{}).Verify(context.Background(), s)
	if !errors.Is(err, ErrAuth) || !strings.Contains(err.Error(), "请改用会话 Cookie") {
		t.Fatalf("got %v", err)
	}
}

func TestNewAPIProbeLoginCaptcha(t *testing.T) {
	for _, on := range []bool{true, false} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			envelope(w, map[string]any{"quota_per_unit": 500000, "version": "v1", "turnstile_check": on})
		}))
		r, err := newAPIAdapter{}.Probe(context.Background(), Site{BaseURL: srv.URL})
		srv.Close()
		if err != nil || r.LoginCaptcha != on || strings.HasSuffix(r.Evidence, "；登录需要人机验证") != on {
			t.Fatalf("turnstile=%v: got %+v err=%v", on, r, err)
		}
		b, _ := json.Marshal(r)
		if !strings.Contains(string(b), `"login_captcha"`) {
			t.Fatalf("json lacks login_captcha: %s", b)
		}
	}
}
