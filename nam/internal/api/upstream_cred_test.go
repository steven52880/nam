package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/steven52880/nam/internal/crypto"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/internal/upstream"
)

func newCredTestServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "nam.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	box, err := crypto.New("k")
	if err != nil {
		t.Fatal(err)
	}
	return &Server{Deps: Deps{Store: st, Box: box}}
}

func sp(s string) *string { return &s }
func ip(i int) *int       { return &i }

func isInvalid(err error) bool {
	var ve *resp.ValidationError
	return errors.As(err, &ve)
}

func apiCookie(ts int64) string {
	return base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("%d|xxx|yyy", ts)))
}

func TestApplyInputRejectsBadSessionCreds(t *testing.T) {
	s := newCredTestServer(t)
	cases := map[string]struct {
		typ, ct string
		uid     *int
		cred    *Credential
		wantMsg string
	}{
		"cookie without user id on newapi": {store.UpstreamNewAPI, store.CredCookie, nil, &Credential{Cookie: "abc"}, "会话 Cookie 需要填写用户 ID"},
		"cookie on sub2api":                {store.UpstreamSub2API, store.CredCookie, ip(1), &Credential{Cookie: "abc"}, "会话 Cookie 只能用于 NewAPI / OneHub 上游"},
		"cookie illegal chars":             {store.UpstreamNewAPI, store.CredCookie, ip(1), &Credential{Cookie: "a b"}, "Cookie 含非法字符"},
		"cookie empty":                     {store.UpstreamNewAPI, store.CredCookie, ip(1), &Credential{Username: "x"}, "请填写会话 Cookie"},
		"session missing refresh":          {store.UpstreamSub2API, store.CredSub2APISession, nil, &Credential{AccessToken: "a"}, "refresh_token"},
		"session missing access":           {store.UpstreamSub2API, store.CredSub2APISession, nil, &Credential{RefreshToken: "r"}, "access_token"},
		"session on newapi":                {store.UpstreamNewAPI, store.CredSub2APISession, nil, &Credential{AccessToken: "a", RefreshToken: "r"}, "只能用于 sub2api"},
	}
	for name, c := range cases {
		u := store.Upstream{CurrencyRate: 1}
		in := UpstreamInput{Name: sp("x"), Type: sp(c.typ), BaseURL: sp("https://a.example"), CredType: sp(c.ct), Credential: c.cred, PlatformUserID: c.uid}
		err := s.applyUpstreamInput(&u, in)
		if !isInvalid(err) || !strings.Contains(err.Error(), c.wantMsg) {
			t.Errorf("%s: got %v", name, err)
		}
		if _, err := siteFromInput(in); !isInvalid(err) || !strings.Contains(err.Error(), c.wantMsg) {
			t.Errorf("%s (draft): got %v", name, err)
		}
	}
}

func TestApplyInputCookieStoresCleanedValueAndUpdatesTimestamp(t *testing.T) {
	s := newCredTestServer(t)
	issued := time.Now().Add(-24 * time.Hour).Unix()
	val := apiCookie(issued)
	u := store.Upstream{CurrencyRate: 1}
	before := time.Now().Unix()
	in := UpstreamInput{
		Name: sp("n"), Type: sp(store.UpstreamNewAPI), BaseURL: sp("https://a.example"), CredType: sp(store.CredCookie),
		Credential: &Credential{Cookie: "a=1; session=" + val + "; b=2", ExpiresAt: 42}, PlatformUserID: ip(7),
	}
	if err := s.applyUpstreamInput(&u, in); err != nil {
		t.Fatal(err)
	}
	if u.CredUpdatedAt < before {
		t.Fatalf("cred_updated_at=%d, want >= %d", u.CredUpdatedAt, before)
	}
	pt, err := s.Box.Open(u.Cred)
	if err != nil {
		t.Fatal(err)
	}
	var got upstream.Credential
	if err := json.Unmarshal(pt, &got); err != nil {
		t.Fatal(err)
	}
	if got.Cookie != val || got.ExpiresAt != 0 {
		t.Fatalf("stored credential %+v", got)
	}
	if err := s.Store.SaveUpstream(context.Background(), &u); err != nil {
		t.Fatal(err)
	}
	v := s.view(u, nil, nil)
	want := issued + int64(upstream.SessionCookieTTL/time.Second)
	if v.CredExpiresAt != want {
		t.Fatalf("cred_expires_at=%d want %d", v.CredExpiresAt, want)
	}
	b, _ := json.Marshal(v)
	if !strings.Contains(string(b), `"cred_expires_at"`) || !strings.Contains(string(b), `"cred_updated_at"`) || strings.Contains(string(b), val) {
		t.Fatalf("view json: %s", b)
	}
}

func TestApplyInputSub2APISessionComputesExpiry(t *testing.T) {
	s := newCredTestServer(t)
	exp := time.Now().Add(2 * time.Hour).Unix()
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, exp)))
	jwt := "h." + payload + ".s"
	u := store.Upstream{CurrencyRate: 1}
	in := UpstreamInput{
		Name: sp("n"), Type: sp(store.UpstreamSub2API), BaseURL: sp("https://a.example"), CredType: sp(store.CredSub2APISession),
		Credential: &Credential{AccessToken: "Bearer " + jwt, RefreshToken: " r1 ", ExpiresAt: 1},
	}
	if err := s.applyUpstreamInput(&u, in); err != nil {
		t.Fatal(err)
	}
	pt, _ := s.Box.Open(u.Cred)
	var got upstream.Credential
	_ = json.Unmarshal(pt, &got)
	if got.AccessToken != jwt || got.RefreshToken != "r1" || got.ExpiresAt != exp {
		t.Fatalf("stored credential %+v", got)
	}
	if v := s.view(u, nil, nil); v.CredExpiresAt != 0 {
		t.Fatalf("sub2api_session cred_expires_at=%d, want 0", v.CredExpiresAt)
	}

	// An unparsable access token is accepted and counts as already expired.
	in.Credential = &Credential{AccessToken: "opaque", RefreshToken: "r1"}
	if err := s.applyUpstreamInput(&u, in); err != nil {
		t.Fatal(err)
	}
	pt, _ = s.Box.Open(u.Cred)
	got = upstream.Credential{}
	_ = json.Unmarshal(pt, &got)
	if got.ExpiresAt != 0 {
		t.Fatalf("expires_at=%d, want 0", got.ExpiresAt)
	}
}

func TestViewCookieExpiryFallsBackToCredUpdatedAt(t *testing.T) {
	s := newCredTestServer(t)
	sealed, _ := s.Box.Seal([]byte(`{"cookie":"opaque"}`))
	u := store.Upstream{Type: store.UpstreamOneHub, CredType: store.CredCookie, Cred: sealed, CredUpdatedAt: 1800000000}
	if got, want := s.view(u, nil, nil).CredExpiresAt, int64(1800000000)+int64(upstream.SessionCookieTTL/time.Second); got != want {
		t.Fatalf("got %d want %d", got, want)
	}
	// Undecryptable credentials must not break listing.
	u.Cred = []byte("garbage-not-sealed-garbage-not-sealed")
	if got := s.view(u, nil, nil).CredExpiresAt; got != 0 {
		t.Fatalf("got %d, want 0", got)
	}
	// OneHub cookie does not need a user id.
	in := UpstreamInput{
		Name: sp("o"), Type: sp(store.UpstreamOneHub), BaseURL: sp("https://a.example"), CredType: sp(store.CredCookie),
		Credential: &Credential{Cookie: "session=abc"},
	}
	if err := s.applyUpstreamInput(&store.Upstream{CurrencyRate: 1}, in); err != nil {
		t.Fatalf("onehub cookie without user id: %v", err)
	}
}

func TestSiteFromInputDraftHasNoCredStore(t *testing.T) {
	site, err := siteFromInput(UpstreamInput{
		Type: sp(store.UpstreamSub2API), BaseURL: sp("https://a.example"), CredType: sp(store.CredSub2APISession),
		Credential: &Credential{AccessToken: "a", RefreshToken: "r"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if site.Creds != nil || site.Cred.RefreshToken != "r" {
		t.Fatalf("site %+v", site)
	}
}
