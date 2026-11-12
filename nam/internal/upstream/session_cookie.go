package upstream

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/steven52880/nam/internal/store"
)

// SessionCookieTTL is the NewAPI session lifetime (MaxAge 2592000 in main.go).
const SessionCookieTTL = 30 * 24 * time.Hour

const sessionCookieName = "session"

// NormalizeSessionCookie accepts a bare value, "session=value" or a whole Cookie
// header ("a=1; session=value; b=2") and returns the session value only.
func NormalizeSessionCookie(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if strings.Contains(v, ";") || strings.Contains(v, "=") {
		found := false
		for _, part := range strings.Split(v, ";") {
			part = strings.TrimSpace(part)
			name, val, ok := strings.Cut(part, "=")
			if ok && strings.TrimSpace(name) == sessionCookieName {
				v, found = strings.TrimSpace(val), true
				break
			}
		}
		if !found {
			// A bare securecookie value is base64url and never contains ";".
			// A value padded with "=" is still a bare value, so only reject
			// strings that look like cookie pairs.
			if strings.Contains(v, ";") || strings.Contains(strings.TrimRight(v, "="), "=") {
				return "", errors.New("没有找到名为 session 的 Cookie")
			}
		}
	}
	if v == "" {
		return "", errors.New("Cookie 含非法字符")
	}
	for i := 0; i < len(v); i++ {
		b := v[i]
		if b < 0x21 || b == 0x7f || b == ';' || b == ',' || b == '"' || b == '\\' {
			return "", errors.New("Cookie 含非法字符")
		}
	}
	return v, nil
}

// SessionCookieIssuedAt decodes the issue time of a gorilla/securecookie value:
// base64url("<unix ts>|<base64 data>|<mac>").
func SessionCookieIssuedAt(v string) (time.Time, bool) {
	raw, err := base64.URLEncoding.DecodeString(v)
	if err != nil {
		raw, err = base64.RawURLEncoding.DecodeString(v)
		if err != nil {
			return time.Time{}, false
		}
	}
	first, _, ok := strings.Cut(string(raw), "|")
	if !ok {
		return time.Time{}, false
	}
	sec, err := strconv.ParseInt(first, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	t := time.Unix(sec, 0)
	if t.Before(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) || t.After(time.Now().Add(24*time.Hour)) {
		return time.Time{}, false
	}
	return t, true
}

// CookieExpiresAt is the estimated expiry (Unix seconds) of a cookie credential:
// issue time + 30 days, else credUpdatedAt + 30 days, else 0.
func CookieExpiresAt(c Credential, credUpdatedAt int64) int64 {
	if t, ok := SessionCookieIssuedAt(c.Cookie); ok {
		return t.Add(SessionCookieTTL).Unix()
	}
	if credUpdatedAt > 0 {
		return credUpdatedAt + int64(SessionCookieTTL/time.Second)
	}
	return 0
}

// captchaHint explains a password-login failure caused by a captcha/Turnstile.
func captchaHint(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "turnstile") || strings.Contains(msg, "验证码") {
		return fmt.Errorf("%w（该站开启了人机验证，账号密码无法登录，请改用会话 Cookie）", err)
	}
	return err
}

// AuthErrorText is the text stored as last_sync_error. Auth failures of session
// credentials get a hint about what to do.
func AuthErrorText(credType string, err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrAuth) {
		switch credType {
		case store.CredCookie:
			return "会话 Cookie 已失效，请在浏览器重新登录后粘贴：" + err.Error()
		case store.CredSub2APISession:
			return "登录令牌已失效，请重新登录后粘贴：" + err.Error()
		}
	}
	return err.Error()
}
