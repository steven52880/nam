package upstream

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/steven52880/nam/internal/store"
)

// memCreds is an in-memory CredStore.
type memCreds struct {
	mu      sync.Mutex
	cred    Credential
	saves   int
	saveErr error
}

func (m *memCreds) Load(context.Context) (Credential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cred, nil
}

func (m *memCreds) Save(_ context.Context, c Credential) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveErr != nil {
		return m.saveErr
	}
	m.cred = c
	m.saves++
	return nil
}

func fakeJWT(exp int64) string {
	enc := base64.RawURLEncoding
	payload, _ := json.Marshal(map[string]any{"exp": exp, "sub": "1"})
	return enc.EncodeToString([]byte(`{"alg":"HS256"}`)) + "." + enc.EncodeToString(payload) + ".sig"
}

// s2RefreshFake serves /auth/refresh and /auth/me. Mode "ok" rotates the pair,
// "401" answers HTTP 401, "code" answers a non-zero envelope code.
type s2RefreshFake struct {
	mu      sync.Mutex
	hits    int
	mode    string
	delay   time.Duration
	wantRef string
	access  string // currently valid access token
}

func (f *s2RefreshFake) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			RefreshToken string `json:"refresh_token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		f.mu.Lock()
		f.hits++
		n, mode, want := f.hits, f.mode, f.wantRef
		f.mu.Unlock()
		time.Sleep(f.delay)
		switch {
		case mode == "401":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":401,"message":"invalid refresh token"}`))
		case mode == "code":
			_, _ = w.Write([]byte(`{"code":4001,"message":"refresh token revoked"}`))
		case want != "" && in.RefreshToken != want:
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":401,"message":"stale refresh token"}`))
		default:
			tok := fakeJWT(time.Now().Add(time.Hour).Unix())
			f.mu.Lock()
			f.access = tok
			f.mu.Unlock()
			s2Reply(w, map[string]any{"access_token": tok, "refresh_token": fmt.Sprintf("r%d", n+1), "expires_in": 3600})
		}
	})
	mux.HandleFunc("/api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		ok := f.access != "" && r.Header.Get("Authorization") == "Bearer "+f.access
		f.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":401,"message":"unauthorized"}`))
			return
		}
		s2Reply(w, map[string]any{"id": 9, "balance": 1.5})
	})
	return mux
}

func (f *s2RefreshFake) hitCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits
}

func s2SessionSite(url string, id uint, c Credential, cs CredStore) Site {
	return Site{ID: id, Type: store.UpstreamSub2API, BaseURL: url, CredType: store.CredSub2APISession, Cred: c, Creds: cs}
}

func TestJWTExpiry(t *testing.T) {
	if got, ok := JWTExpiry(fakeJWT(1900000000)); !ok || got != 1900000000 {
		t.Fatalf("got %d ok=%v", got, ok)
	}
	padded := "h." + base64.URLEncoding.EncodeToString([]byte(`{"exp":1900000001}`)) + ".s"
	if got, ok := JWTExpiry(padded); !ok || got != 1900000001 {
		t.Fatalf("padded: got %d ok=%v", got, ok)
	}
	noExp := "h." + base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"1"}`)) + ".s"
	for name, tok := range map[string]string{"garbage": "not-a-jwt", "empty": "", "no exp": noExp, "bad b64": "h.!!!.s"} {
		if _, ok := JWTExpiry(tok); ok {
			t.Errorf("%s: want failure", name)
		}
	}
}

func TestS2SessionFreshTokenNotRefreshed(t *testing.T) {
	f := &s2RefreshFake{}
	tok := fakeJWT(time.Now().Add(time.Hour).Unix())
	f.access = tok
	srv := httptest.NewServer(f.handler())
	defer srv.Close()
	cred := Credential{AccessToken: tok, RefreshToken: "r1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	mc := &memCreds{cred: cred}
	if err := (sub2apiAdapter{}).Verify(context.Background(), s2SessionSite(srv.URL, 9101, cred, mc)); err != nil {
		t.Fatal(err)
	}
	if f.hitCount() != 0 || mc.saves != 0 {
		t.Fatalf("hits=%d saves=%d, want none", f.hitCount(), mc.saves)
	}
}

func TestS2SessionRefreshesOnceAndPersists(t *testing.T) {
	f := &s2RefreshFake{wantRef: "r1"}
	srv := httptest.NewServer(f.handler())
	defer srv.Close()
	cred := Credential{AccessToken: "old", RefreshToken: "r1", ExpiresAt: time.Now().Add(30 * time.Second).Unix(), Username: "keep"}
	mc := &memCreds{cred: cred}
	site := s2SessionSite(srv.URL, 9102, cred, mc)
	if err := (sub2apiAdapter{}).Verify(context.Background(), site); err != nil {
		t.Fatal(err)
	}
	if f.hitCount() != 1 || mc.saves != 1 {
		t.Fatalf("hits=%d saves=%d, want 1/1", f.hitCount(), mc.saves)
	}
	saved, _ := mc.Load(context.Background())
	if saved.RefreshToken != "r2" || saved.AccessToken == "old" || saved.Username != "keep" {
		t.Fatalf("saved credential not rotated correctly: %+v", saved)
	}
	if saved.ExpiresAt <= time.Now().Add(50*time.Minute).Unix() {
		t.Fatalf("expires_at not advanced: %d", saved.ExpiresAt)
	}
	// A second call with the stale in-memory Site must reuse the persisted pair.
	if err := (sub2apiAdapter{}).Verify(context.Background(), site); err != nil {
		t.Fatal(err)
	}
	if f.hitCount() != 1 {
		t.Fatalf("hits=%d after second call, want 1", f.hitCount())
	}
}

func TestS2SessionConcurrentRefreshHitsEndpointOnce(t *testing.T) {
	f := &s2RefreshFake{wantRef: "r1", delay: 100 * time.Millisecond}
	srv := httptest.NewServer(f.handler())
	defer srv.Close()
	cred := Credential{AccessToken: "old", RefreshToken: "r1", ExpiresAt: time.Now().Add(-time.Minute).Unix()}
	mc := &memCreds{cred: cred}
	site := s2SessionSite(srv.URL, 9103, cred, mc)

	var wg sync.WaitGroup
	toks := make([]string, 2)
	errs := make([]error, 2)
	start := make(chan struct{})
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			toks[i], errs[i] = s2EnsureAccess(context.Background(), site)
		}()
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
	}
	if f.hitCount() != 1 {
		t.Fatalf("refresh endpoint hit %d times, want exactly 1", f.hitCount())
	}
	if toks[0] == "" || toks[0] != toks[1] || toks[0] == "old" {
		t.Fatalf("tokens differ or stale: %q vs %q", toks[0], toks[1])
	}
	if mc.saves != 1 {
		t.Fatalf("saves=%d, want 1", mc.saves)
	}
}

func TestS2SessionUsesFreshlyPersistedPairAfterLock(t *testing.T) {
	f := &s2RefreshFake{}
	srv := httptest.NewServer(f.handler())
	defer srv.Close()
	fresh := fakeJWT(time.Now().Add(time.Hour).Unix())
	mc := &memCreds{cred: Credential{AccessToken: fresh, RefreshToken: "r9", ExpiresAt: time.Now().Add(time.Hour).Unix()}}
	stale := Credential{AccessToken: "old", RefreshToken: "r1", ExpiresAt: time.Now().Add(-time.Minute).Unix()}
	got, err := s2EnsureAccess(context.Background(), s2SessionSite(srv.URL, 9104, stale, mc))
	if err != nil || got != fresh {
		t.Fatalf("got %q err=%v", got, err)
	}
	if f.hitCount() != 0 {
		t.Fatalf("hits=%d, want 0", f.hitCount())
	}
}

func TestS2SessionRefreshRejectedIsErrAuth(t *testing.T) {
	for _, mode := range []string{"401", "code"} {
		f := &s2RefreshFake{mode: mode}
		srv := httptest.NewServer(f.handler())
		cred := Credential{AccessToken: "old", RefreshToken: "secret-refresh", ExpiresAt: time.Now().Add(-time.Minute).Unix()}
		mc := &memCreds{cred: cred}
		_, err := s2EnsureAccess(context.Background(), s2SessionSite(srv.URL, 9105, cred, mc))
		srv.Close()
		if !errors.Is(err, ErrAuth) || !strings.Contains(err.Error(), "续期失败") {
			t.Fatalf("mode %s: got %v", mode, err)
		}
		if strings.Contains(err.Error(), "secret-refresh") {
			t.Fatalf("mode %s: refresh token leaked into error", mode)
		}
		if mc.saves != 0 {
			t.Fatalf("mode %s: credential saved after failed refresh", mode)
		}
	}
}

func TestS2SessionNoStoreAndExpiredIsErrAuthWithoutRefresh(t *testing.T) {
	f := &s2RefreshFake{}
	srv := httptest.NewServer(f.handler())
	defer srv.Close()
	cred := Credential{AccessToken: "old", RefreshToken: "r1", ExpiresAt: time.Now().Add(-time.Minute).Unix()}
	_, err := s2EnsureAccess(context.Background(), s2SessionSite(srv.URL, 9106, cred, nil))
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("got %v", err)
	}
	if f.hitCount() != 0 {
		t.Fatalf("refresh called %d times without a CredStore", f.hitCount())
	}
	if _, err := s2EnsureAccess(context.Background(), s2SessionSite(srv.URL, 9107, Credential{}, nil)); !errors.Is(err, ErrAuth) {
		t.Fatalf("empty credential: got %v", err)
	}
}

func TestS2SessionSaveFailureIsError(t *testing.T) {
	f := &s2RefreshFake{}
	srv := httptest.NewServer(f.handler())
	defer srv.Close()
	cred := Credential{AccessToken: "old", RefreshToken: "r1", ExpiresAt: time.Now().Add(-time.Minute).Unix()}
	mc := &memCreds{cred: cred, saveErr: errors.New("disk full")}
	tok, err := s2EnsureAccess(context.Background(), s2SessionSite(srv.URL, 9108, cred, mc))
	if err == nil || tok != "" || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("tok=%q err=%v", tok, err)
	}
}

func TestS2SessionIncompleteRefreshResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		s2Reply(w, map[string]any{"access_token": "a", "refresh_token": "", "expires_in": 3600})
	}))
	defer srv.Close()
	cred := Credential{AccessToken: "old", RefreshToken: "r1", ExpiresAt: time.Now().Add(-time.Minute).Unix()}
	mc := &memCreds{cred: cred}
	if _, err := s2EnsureAccess(context.Background(), s2SessionSite(srv.URL, 9109, cred, mc)); err == nil || mc.saves != 0 {
		t.Fatalf("err=%v saves=%d", err, mc.saves)
	}
}
