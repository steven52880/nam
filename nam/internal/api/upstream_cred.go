package api

import (
	"strings"

	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/internal/upstream"
)

// checkSessionCred validates the session credential types against the final upstream
// type and user id, and normalises cred in place (cred may be nil when no credential
// is being written). Types other than cookie / sub2api_session pass through, except
// that a client-supplied expires_at is always dropped: the server computes it.
func checkSessionCred(typ, credType string, userID int, cred *Credential) error {
	switch credType {
	case store.CredCookie:
		if typ != store.UpstreamNewAPI && typ != store.UpstreamOneHub {
			return resp.Invalid("会话 Cookie 只能用于 NewAPI / OneHub 上游")
		}
		if cred == nil {
			return nil
		}
		if strings.TrimSpace(cred.Cookie) == "" {
			return resp.Invalid("请填写会话 Cookie")
		}
		v, err := upstream.NormalizeSessionCookie(cred.Cookie)
		if err != nil {
			return resp.Invalid(err.Error())
		}
		cred.Cookie = v
		cred.ExpiresAt = 0
		if typ == store.UpstreamNewAPI && userID <= 0 {
			return resp.Invalid("会话 Cookie 需要填写用户 ID")
		}
	case store.CredSub2APISession:
		if typ != store.UpstreamSub2API {
			return resp.Invalid("登录令牌（自动续期）只能用于 sub2api 上游")
		}
		if cred == nil {
			return nil
		}
		access := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(cred.AccessToken), "Bearer "))
		refresh := strings.TrimSpace(cred.RefreshToken)
		if access == "" || refresh == "" {
			return resp.Invalid("登录令牌需要同时填写 access_token 和 refresh_token")
		}
		cred.AccessToken, cred.RefreshToken = access, refresh
		// Unparsable expiry counts as already expired: the first use renews it.
		cred.ExpiresAt, _ = upstream.JWTExpiry(access)
	default:
		if cred != nil {
			cred.ExpiresAt = 0
		}
	}
	return nil
}
