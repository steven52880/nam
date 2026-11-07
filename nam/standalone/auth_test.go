package standalone

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/steven52880/nam/core"
)

type authEnv struct {
	h    http.Handler
	auth *Auth
	now  time.Time
}

func newAuthEnv(t *testing.T) *authEnv {
	t.Helper()
	f := newFakeNewAPI(t)
	p, _, err := build(f.config(), newClient(f.config(), time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	env := &authEnv{auth: p.Auth.(*Auth), now: time.Now()}
	env.auth.now = func() time.Time { return env.now }
	c, err := core.New(core.Config{DBPath: filepath.Join(t.TempDir(), "nam.db"), Secret: "x"}, p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Stop)
	env.h = c.Handler()
	return env
}

func (e *authEnv) do(method, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
}

func (e *authEnv) login(pw string) *httptest.ResponseRecorder {
	return e.do(http.MethodPost, "/api/nam/auth/login", map[string]string{"password": pw}, nil)
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == cookieName && c.Value != "" {
			return c
		}
	}
	t.Fatalf("no %s cookie: %s", cookieName, rec.Body.String())
	return nil
}

type failBody struct {
	Message string `json:"message"`
	Data    struct {
		Failures    int   `json:"failures"`
		LockedUntil int64 `json:"locked_until"`
	} `json:"data"`
}

func decodeFail(t *testing.T, rec *httptest.ResponseRecorder) failBody {
	t.Helper()
	var b failBody
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return b
}

func TestLoginWrongPasswordCountsFailures(t *testing.T) {
	e := newAuthEnv(t)
	rec := e.login("nope")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
	if b := decodeFail(t, rec); b.Message != "密码错误" || b.Data.Failures != 1 {
		t.Fatalf("body %+v", b)
	}
}

func TestLoginLocksAfterFiveFailures(t *testing.T) {
	e := newAuthEnv(t)
	for i := 1; i <= 5; i++ {
		if rec := e.login("nope"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status %d", i, rec.Code)
		}
	}
	rec := e.login("initial-pass") // even the right password is refused while locked
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("locked status %d, want 429", rec.Code)
	}
	if decodeFail(t, rec).Data.LockedUntil <= e.now.Unix() {
		t.Fatal("locked_until must be in the future")
	}
	e.now = e.now.Add(lockDuration + time.Second)
	if rec := e.login("initial-pass"); rec.Code != http.StatusOK {
		t.Fatalf("after lock expiry status %d", rec.Code)
	}
}

func TestSessionGatesPrivateRoutes(t *testing.T) {
	e := newAuthEnv(t)
	if rec := e.do(http.MethodGet, "/api/nam/upstreams", nil, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no cookie: %d", rec.Code)
	}
	rec := e.login("initial-pass")
	if rec.Code != http.StatusOK {
		t.Fatalf("login %d: %s", rec.Code, rec.Body.String())
	}
	ck := sessionCookie(t, rec)
	if !ck.HttpOnly || ck.SameSite != http.SameSiteLaxMode || ck.Path != "/" || ck.Secure {
		t.Fatalf("cookie attrs: %+v", ck)
	}
	if rec := e.do(http.MethodGet, "/api/nam/upstreams", nil, ck); rec.Code != http.StatusOK {
		t.Fatalf("with cookie: %d %s", rec.Code, rec.Body.String())
	}
	e.do(http.MethodPost, "/api/nam/auth/logout", nil, ck)
	if rec := e.do(http.MethodGet, "/api/nam/upstreams", nil, ck); rec.Code != http.StatusUnauthorized {
		t.Fatalf("after logout: %d", rec.Code)
	}
}

func TestSessionsSurviveRestart(t *testing.T) {
	e := newAuthEnv(t)
	ck := sessionCookie(t, e.login("initial-pass"))
	e.auth.mu.Lock()
	e.auth.sessions, e.auth.loaded = map[string]int64{}, false // simulate a fresh process
	e.auth.mu.Unlock()
	if rec := e.do(http.MethodGet, "/api/nam/upstreams", nil, ck); rec.Code != http.StatusOK {
		t.Fatalf("persisted session rejected: %d", rec.Code)
	}
}

func TestChangePasswordInvalidatesSessions(t *testing.T) {
	e := newAuthEnv(t)
	ck := sessionCookie(t, e.login("initial-pass"))
	bad := map[string]string{"old_password": "initial-pass", "new_password": "short", "confirm_password": "short"}
	if rec := e.do(http.MethodPost, "/api/nam/auth/password", bad, ck); rec.Code != http.StatusBadRequest {
		t.Fatalf("short password: %d", rec.Code)
	}
	body := map[string]string{"old_password": "initial-pass", "new_password": "brand-new-pass", "confirm_password": "brand-new-pass"}
	if rec := e.do(http.MethodPost, "/api/nam/auth/password", body, ck); rec.Code != http.StatusOK {
		t.Fatalf("change: %d %s", rec.Code, rec.Body.String())
	}
	if rec := e.do(http.MethodGet, "/api/nam/upstreams", nil, ck); rec.Code != http.StatusUnauthorized {
		t.Fatalf("old session must be dead: %d", rec.Code)
	}
	if rec := e.login("initial-pass"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("old password must fail: %d", rec.Code)
	}
	if rec := e.login("brand-new-pass"); rec.Code != http.StatusOK {
		t.Fatalf("new password must work: %d", rec.Code)
	}
}

func TestLoginWithoutAnyPasswordFails(t *testing.T) {
	f := newFakeNewAPI(t)
	cfg := f.config()
	cfg.AdminPassword = ""
	a := NewAuth(cfg)
	h, err := a.hashLocked(t.Context())
	if err != nil || h != "" {
		t.Fatalf("hash=%q err=%v", h, err)
	}
}
