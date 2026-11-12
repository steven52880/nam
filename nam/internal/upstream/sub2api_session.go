package upstream

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// s2Locks serialises token refreshes per upstream id (uint -> *sync.Mutex).
// sub2api invalidates the old refresh token on every refresh, so two concurrent
// refreshes would burn the credential.
var s2Locks sync.Map

// s2RefreshSkew is how long before expiry the access token is already renewed.
const s2RefreshSkew = 2 * time.Minute

// JWTExpiry reads the numeric "exp" claim (Unix seconds) of a JWT without verifying it.
func JWTExpiry(tok string) (int64, bool) {
	parts := strings.Split(strings.TrimSpace(tok), ".")
	if len(parts) < 2 {
		return 0, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return 0, false
	}
	var claims struct {
		Exp *float64 `json:"exp"`
	}
	if json.Unmarshal(raw, &claims) != nil || claims.Exp == nil || *claims.Exp <= 0 {
		return 0, false
	}
	return int64(*claims.Exp), true
}

func s2Fresh(c Credential) bool {
	return c.AccessToken != "" && c.ExpiresAt > time.Now().Add(s2RefreshSkew).Unix()
}

func s2Bare(tok string) string { return strings.TrimPrefix(strings.TrimSpace(tok), "Bearer ") }

// s2EnsureAccess returns a usable access token for a sub2api_session credential,
// refreshing (and persisting the rotated pair) when it is expired or about to be.
func s2EnsureAccess(ctx context.Context, s Site) (string, error) {
	if s.Cred.AccessToken == "" && s.Cred.RefreshToken == "" {
		return "", fmt.Errorf("access token and refresh token missing: %w", ErrAuth)
	}
	if s2Fresh(s.Cred) {
		return s2Bare(s.Cred.AccessToken), nil
	}
	if s.Creds == nil {
		return "", fmt.Errorf("登录令牌已过期，保存上游后会自动续期: %w", ErrAuth)
	}
	m, _ := s2Locks.LoadOrStore(s.ID, &sync.Mutex{})
	mu, _ := m.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	// Another caller may have refreshed while we waited for the lock.
	cur, err := s.Creds.Load(ctx)
	if err != nil {
		return "", fmt.Errorf("load credential: %w", err)
	}
	if s2Fresh(cur) {
		return s2Bare(cur.AccessToken), nil
	}
	if strings.TrimSpace(cur.RefreshToken) == "" {
		return "", fmt.Errorf("refresh token missing: %w", ErrAuth)
	}
	s.Cred = cur
	next, err := s2Refresh(ctx, s, cur.RefreshToken)
	if err != nil {
		return "", err
	}
	// The old refresh token is already dead: persist before handing out the new one.
	if err := s.Creds.Save(ctx, next); err != nil {
		return "", fmt.Errorf("save refreshed credential: %w", err)
	}
	return next.AccessToken, nil
}

// s2Refresh calls POST /api/v1/auth/refresh and returns s.Cred with the new token pair.
func s2Refresh(ctx context.Context, s Site, refresh string) (Credential, error) {
	c := &s2Session{hc: s.client(), base: s.BaseURL, hdr: map[string]string{}}
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	err := c.call(ctx, http.MethodPost, "/api/v1/auth/refresh", map[string]string{"refresh_token": refresh}, &out)
	if err != nil {
		var ae *apiError
		switch {
		case errors.Is(err, ErrAuth):
			return Credential{}, fmt.Errorf("续期失败：%s: %w", err.Error(), ErrAuth)
		case errors.As(err, &ae):
			return Credential{}, fmt.Errorf("续期失败：%s: %w", ae.msg, ErrAuth)
		}
		return Credential{}, fmt.Errorf("refresh: %w", err)
	}
	if out.AccessToken == "" || out.RefreshToken == "" || out.ExpiresIn <= 0 {
		return Credential{}, errors.New("refresh returned an incomplete token pair")
	}
	next := s.Cred
	next.AccessToken = out.AccessToken
	next.RefreshToken = out.RefreshToken
	next.ExpiresAt = time.Now().Unix() + out.ExpiresIn
	slog.Info("nam: sub2api token refreshed", "upstream", s.ID)
	return next, nil
}
