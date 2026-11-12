package cachetest

import (
	"context"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"

	"github.com/steven52880/nam/internal/crypto"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

// DefaultEmbeddedURL is the NewAPI address used when nothing is configured and the
// backend reports none (embedded mode: nam runs inside NewAPI's process).
const DefaultEmbeddedURL = "http://127.0.0.1:3000"

// ErrNoToken means the test token was never configured.
var ErrNoToken = resp.Invalid("尚未配置「NewAPI 测试令牌」：请到设置页填写一个管理员账号名下的 NewAPI 令牌")

// CleanToken validates a pasted token and returns it without the leading "sk-".
func CleanToken(raw string) (string, error) {
	t := strings.TrimPrefix(strings.TrimSpace(raw), "sk-")
	switch {
	case t == "":
		return "", resp.Invalid("NewAPI 测试令牌不能为空")
	case strings.ContainsAny(t, " \t\r\n,"):
		return "", resp.Invalid("NewAPI 测试令牌不能包含空白或逗号")
	case strings.Contains(t, "-"):
		return "", resp.Invalid("NewAPI 测试令牌格式不正确：去掉 sk- 前缀后不应再含有 “-”（指定渠道用 “-” 分隔渠道 ID）")
	}
	return t, nil
}

// TokenHint is the display form of a token: first 5 + … + last 4 of "sk-<token>".
func TokenHint(token string) string {
	r := []rune("sk-" + token)
	if len(r) <= 9 {
		return string(r[:2]) + "…"
	}
	return string(r[:5]) + "…" + string(r[len(r)-4:])
}

// SaveToken seals and stores the token plus its hint. An empty raw value clears both.
func SaveToken(ctx context.Context, st *store.Store, box *crypto.Box, raw string) error {
	if strings.TrimSpace(raw) == "" {
		if err := st.SetKV(ctx, store.KeyCacheTestToken, ""); err != nil {
			return err
		}
		return st.SetKV(ctx, store.KeyCacheTestTokenHint, "")
	}
	tok, err := CleanToken(raw)
	if err != nil {
		return err
	}
	sealed, err := box.Seal([]byte(tok))
	if err != nil {
		return err
	}
	if err := st.SetKV(ctx, store.KeyCacheTestToken, base64.StdEncoding.EncodeToString(sealed)); err != nil {
		return err
	}
	return st.SetKV(ctx, store.KeyCacheTestTokenHint, TokenHint(tok))
}

// LoadToken returns the decrypted token (without sk-) or ErrNoToken.
func LoadToken(ctx context.Context, st *store.Store, box *crypto.Box) (string, error) {
	v, found, err := st.GetKV(ctx, store.KeyCacheTestToken)
	if err != nil {
		return "", err
	}
	if !found || v == "" {
		return "", ErrNoToken
	}
	blob, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return "", errors.New("NewAPI 测试令牌已损坏，请重新填写")
	}
	pt, err := box.Open(blob)
	if err != nil {
		return "", errors.New("NewAPI 测试令牌无法解密（NAM_SECRET 是否变了？），请重新填写")
	}
	return string(pt), nil
}

// NormalizeURL validates a NewAPI address setting and strips trailing slashes.
// An empty value is allowed (means "use the default").
func NormalizeURL(raw string) (string, error) {
	v := strings.TrimRight(strings.TrimSpace(raw), "/")
	if v == "" {
		return "", nil
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", resp.Invalid("NewAPI 地址必须以 http:// 或 https:// 开头，例如 http://127.0.0.1:3000")
	}
	return v, nil
}

// ResolveBaseURL picks the relay address: the setting if set; else the backend's own
// address (standalone: NAM_NEWAPI_URL); else DefaultEmbeddedURL.
func ResolveBaseURL(ctx context.Context, st *store.Store, backend ports.Backend) string {
	if v, found, err := st.GetKV(ctx, store.KeyNewAPIURL); err == nil && found && strings.TrimSpace(v) != "" {
		return strings.TrimRight(strings.TrimSpace(v), "/")
	}
	if backend != nil {
		// Info may return an error (NewAPI unreachable) but still carries the address.
		if info, _ := backend.Info(ctx); strings.TrimSpace(info.Address) != "" {
			return strings.TrimRight(strings.TrimSpace(info.Address), "/")
		}
	}
	return DefaultEmbeddedURL
}
