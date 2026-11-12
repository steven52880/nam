package upstream

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/steven52880/nam/internal/store"
)

func TestNormalizeSessionCookieFormats(t *testing.T) {
	for _, in := range []string{
		"MTcwMDAwMDAwMHx4eHh8eXl5",
		"  MTcwMDAwMDAwMHx4eHh8eXl5\n",
		"session=MTcwMDAwMDAwMHx4eHh8eXl5",
		"a=1; session=MTcwMDAwMDAwMHx4eHh8eXl5; b=2",
		"session=MTcwMDAwMDAwMHx4eHh8eXl5;",
	} {
		got, err := NormalizeSessionCookie(in)
		if err != nil || got != "MTcwMDAwMDAwMHx4eHh8eXl5" {
			t.Errorf("%q: got %q err=%v", in, got, err)
		}
	}
	// A base64 value padded with "=" is still a bare value.
	if got, err := NormalizeSessionCookie("abc=="); err != nil || got != "abc==" {
		t.Errorf("padded: got %q err=%v", got, err)
	}
}

func TestNormalizeSessionCookieRejects(t *testing.T) {
	for _, in := range []string{"", "   ", "session=", "ab cd", "ab,cd", `ab"cd`, `ab\cd`, "ab\x7fcd"} {
		if _, err := NormalizeSessionCookie(in); err == nil {
			t.Errorf("%q: want error", in)
		}
	}
	_, err := NormalizeSessionCookie("a=1; b=2")
	if err == nil || !strings.Contains(err.Error(), "没有找到名为 session 的 Cookie") {
		t.Errorf("no session pair: got %v", err)
	}
	_, err = NormalizeSessionCookie("ab cd")
	if err == nil || !strings.Contains(err.Error(), "Cookie 含非法字符") {
		t.Errorf("illegal char: got %v", err)
	}
}

func cookieAt(ts int64) string {
	return base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("%d|xxx|yyy", ts)))
}

func TestSessionCookieIssuedAt(t *testing.T) {
	got, ok := SessionCookieIssuedAt(cookieAt(1700000000))
	if !ok || got.Unix() != 1700000000 {
		t.Fatalf("got %v ok=%v", got, ok)
	}
	raw := base64.RawURLEncoding.EncodeToString([]byte("1700000000|xxx|yyy"))
	if got, ok := SessionCookieIssuedAt(raw); !ok || got.Unix() != 1700000000 {
		t.Fatalf("raw encoding: got %v ok=%v", got, ok)
	}
	for name, v := range map[string]string{
		"garbage":   "!!!not-base64!!!",
		"no pipe":   base64.URLEncoding.EncodeToString([]byte("1700000000")),
		"not a num": base64.URLEncoding.EncodeToString([]byte("abc|x|y")),
		"too old":   cookieAt(1500000000),
		"future":    cookieAt(time.Now().Add(48 * time.Hour).Unix()),
		"empty":     "",
	} {
		if _, ok := SessionCookieIssuedAt(v); ok {
			t.Errorf("%s: want failure", name)
		}
	}
}

func TestCookieExpiresAt(t *testing.T) {
	ttl := int64(SessionCookieTTL / time.Second)
	if got := CookieExpiresAt(Credential{Cookie: cookieAt(1700000000)}, 1800000000); got != 1700000000+ttl {
		t.Errorf("from issue time: got %d", got)
	}
	if got := CookieExpiresAt(Credential{Cookie: "opaque"}, 1800000000); got != 1800000000+ttl {
		t.Errorf("from cred_updated_at: got %d", got)
	}
	if got := CookieExpiresAt(Credential{Cookie: "opaque"}, 0); got != 0 {
		t.Errorf("unknown: got %d", got)
	}
}

func TestCaptchaHint(t *testing.T) {
	base := fmt.Errorf("login failed: Turnstile token is empty: %w", ErrAuth)
	got := captchaHint(base)
	if !errors.Is(got, ErrAuth) || !strings.Contains(got.Error(), "请改用会话 Cookie") {
		t.Fatalf("turnstile: %v", got)
	}
	cn := fmt.Errorf("login failed: 验证码错误: %w", ErrAuth)
	if got := captchaHint(cn); !strings.Contains(got.Error(), "请改用会话 Cookie") {
		t.Fatalf("验证码: %v", got)
	}
	plain := fmt.Errorf("login failed: bad password: %w", ErrAuth)
	if got := captchaHint(plain); got.Error() != plain.Error() {
		t.Fatalf("plain error changed: %v", got)
	}
}

func TestAuthErrorText(t *testing.T) {
	authErr := fmt.Errorf("HTTP 401: %w", ErrAuth)
	if got := AuthErrorText(store.CredCookie, authErr); !strings.HasPrefix(got, "会话 Cookie 已失效，请在浏览器重新登录后粘贴：") || !strings.HasSuffix(got, authErr.Error()) {
		t.Errorf("cookie: %q", got)
	}
	if got := AuthErrorText(store.CredSub2APISession, authErr); !strings.HasPrefix(got, "登录令牌已失效，请重新登录后粘贴：") {
		t.Errorf("sub2api_session: %q", got)
	}
	if got := AuthErrorText(store.CredPassword, authErr); got != authErr.Error() {
		t.Errorf("password: %q", got)
	}
	other := errors.New("boom")
	if got := AuthErrorText(store.CredCookie, other); got != "boom" {
		t.Errorf("non-auth: %q", got)
	}
}
