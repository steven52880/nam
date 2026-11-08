package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/steven52880/nam/internal/store"
)

// sub2api adapter.
//
// Verified from the local fork F:\Mess\newapi-configurator\temp\clansty-new-api
// (service/sub2api_client.go, service/sub2api_auth.go, controller/channel_balance_query.go):
//   - envelope: {"code":0,"message":"","data":...}; code != 0 or non-2xx is a failure.
//   - POST /api/v1/auth/login {email,password,not_in_cn_confirmed:true} →
//     data {access_token,refresh_token,expires_in,requires_2fa}.
//   - Authorization: Bearer <JWT> on /api/v1/* user routes.
//   - GET /api/v1/keys?search=&page=&page_size= → data.items[{key, group{id,name,description,rate_multiplier}}].
//   - GET /api/v1/groups/rates → data {"<group id>": per-user rate multiplier}.
//
// Everything else (auth/me, announcements, settings/public, key quota fields) comes from the
// task plan only and is marked UNVERIFIED where it is used.

func init() { Register(sub2apiAdapter{}) }

type sub2apiAdapter struct{}

func (sub2apiAdapter) Type() string { return store.UpstreamSub2API }

type s2Session struct {
	hc   *http.Client
	base string
	hdr  map[string]string
}

// s2Envelope is {code,message,data}; Code is a pointer so that non-sub2api JSON is detectable.
type s2Envelope struct {
	Code    *int            `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (c *s2Session) call(ctx context.Context, method, path string, body, out any) error {
	var env s2Envelope
	if err := doJSON(ctx, c.hc, method, joinURL(c.base, path), c.hdr, body, &env); err != nil {
		return err
	}
	if env.Code == nil {
		return errors.New("response is not a sub2api envelope")
	}
	if *env.Code != 0 {
		if *env.Code == http.StatusUnauthorized || *env.Code == http.StatusForbidden {
			return fmt.Errorf("code %d: %s: %w", *env.Code, env.Message, ErrAuth)
		}
		return &apiError{msg: fmt.Sprintf("code %d: %s", *env.Code, env.Message)}
	}
	if out == nil || len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("decode data: %w", err)
	}
	return nil
}

func (c *s2Session) get(ctx context.Context, path string, out any) error {
	return c.call(ctx, http.MethodGet, path, nil, out)
}

// s2Open authenticates once per adapter call: a password credential logs in (no token
// caching), an access_token credential is the JWT itself.
func s2Open(ctx context.Context, s Site) (*s2Session, error) {
	c := &s2Session{hc: s.client(), base: s.BaseURL, hdr: map[string]string{}}
	switch s.CredType {
	case store.CredPassword:
		if strings.TrimSpace(s.Cred.Username) == "" || s.Cred.Password == "" {
			return nil, fmt.Errorf("username and password required: %w", ErrAuth)
		}
		var tok struct {
			AccessToken string `json:"access_token"`
			Requires2FA bool   `json:"requires_2fa"`
		}
		body := map[string]any{"email": s.Cred.Username, "password": s.Cred.Password, "not_in_cn_confirmed": true}
		if err := c.call(ctx, http.MethodPost, "/api/v1/auth/login", body, &tok); err != nil {
			if errors.Is(err, ErrAuth) {
				return nil, fmt.Errorf("login failed: %w", err)
			}
			var ae *apiError
			if errors.As(err, &ae) {
				return nil, fmt.Errorf("login failed: %s: %w", ae.msg, ErrAuth)
			}
			return nil, fmt.Errorf("login: %w", err)
		}
		if tok.Requires2FA {
			return nil, fmt.Errorf("account requires two-factor authentication: %w", ErrAuth)
		}
		if tok.AccessToken == "" {
			return nil, fmt.Errorf("login returned no access token: %w", ErrAuth)
		}
		c.hdr["Authorization"] = "Bearer " + tok.AccessToken
	case store.CredAccessToken:
		if s.Cred.AccessToken == "" {
			return nil, fmt.Errorf("access token missing: %w", ErrAuth)
		}
		c.hdr["Authorization"] = "Bearer " + strings.TrimPrefix(s.Cred.AccessToken, "Bearer ")
	default:
		return nil, fmt.Errorf("cred type %q unusable for sub2api: %w", s.CredType, ErrUnsupported)
	}
	return c, nil
}

// Probe uses GET /api/v1/settings/public. UNVERIFIED: this public route and its optional
// `version` field; detection relies on the {code:0,data:{}} envelope only.
func (sub2apiAdapter) Probe(ctx context.Context, s Site) (ProbeResult, error) {
	c := &s2Session{hc: s.client(), base: s.BaseURL, hdr: map[string]string{}}
	var d struct {
		Version string `json:"version"`
	}
	if err := c.get(ctx, "/api/v1/settings/public", &d); err != nil {
		return ProbeResult{}, fmt.Errorf("probe /api/v1/settings/public: %w", err)
	}
	return ProbeResult{
		Type:     store.UpstreamSub2API,
		Evidence: "/api/v1/settings/public 返回 sub2api 信封 {code:0,data}",
		Version:  d.Version,
	}, nil
}

// s2Me is GET /api/v1/auth/me. UNVERIFIED: balance is the remaining USD of the user
// (sub2api bills in USD; the fork's key-level /v1/usage rejects any unit other than USD).
type s2Me struct {
	ID      int64    `json:"id"`
	Balance *float64 `json:"balance"`
}

func (sub2apiAdapter) Verify(ctx context.Context, s Site) error {
	c, err := s2Open(ctx, s)
	if err != nil {
		return err
	}
	return c.get(ctx, "/api/v1/auth/me", nil)
}

func (sub2apiAdapter) Balance(ctx context.Context, s Site) (Balance, error) {
	c, err := s2Open(ctx, s)
	if err != nil {
		return Balance{}, err
	}
	var me s2Me
	if err := c.get(ctx, "/api/v1/auth/me", &me); err != nil {
		return Balance{}, fmt.Errorf("auth/me: %w", err)
	}
	if me.Balance == nil {
		return Balance{}, errors.New("auth/me has no balance field")
	}
	return Balance{USD: *me.Balance, Raw: fmt.Sprintf("balance=%g", *me.Balance)}, nil
}

func (sub2apiAdapter) Announcements(ctx context.Context, s Site) (Announcement, error) {
	// UNVERIFIED: the route may be public; fall back to anonymous when login is unusable.
	c, err := s2Open(ctx, s)
	if err != nil {
		c = &s2Session{hc: s.client(), base: s.BaseURL, hdr: map[string]string{}}
	}
	var raw json.RawMessage
	if err := c.get(ctx, "/api/v1/announcements", &raw); err != nil {
		return Announcement{}, fmt.Errorf("announcements: %w", err)
	}
	return Announcement{Content: renderS2Announcements(raw)}, nil
}

type s2Notice struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// renderS2Announcements accepts data as [..] or {items:[..]}. UNVERIFIED: item fields title/content.
func renderS2Announcements(raw json.RawMessage) string {
	var list []s2Notice
	if json.Unmarshal(raw, &list) != nil {
		var page struct {
			Items []s2Notice `json:"items"`
		}
		if json.Unmarshal(raw, &page) != nil {
			return ""
		}
		list = page.Items
	}
	parts := make([]string, 0, len(list))
	for _, n := range list {
		text := strings.TrimSpace(strings.TrimSpace(n.Title) + "\n" + strings.TrimSpace(n.Content))
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n")
}

func (sub2apiAdapter) ListModels(ctx context.Context, s Site, key string) ([]string, error) {
	return listModelsOpenAI(ctx, s, key)
}
