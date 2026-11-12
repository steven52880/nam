package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/steven52880/nam/internal/store"
)

// Right Code (rightapi.io) adapter. See docs/rightcode-adapter.md.
//   - Spring Boot backend, no /api prefix, success responses are bare JSON (no envelope).
//   - Auth: Authorization: Bearer <user_token>. Password login needs a Tencent captcha, so
//     only the access_token credential is supported.
//   - A Right Code "channel" (upstream, identified by id + path prefix such as /codex) is the
//     project's "group"; the group name is the prefix.

func init() { Register(rightCodeAdapter{}) }

type rightCodeAdapter struct{}

func (rightCodeAdapter) Type() string { return store.UpstreamRightCode }

type rcSession struct {
	hc   *http.Client
	base string
	hdr  map[string]string
}

func rcOpen(s Site) (*rcSession, error) {
	if s.CredType != store.CredAccessToken {
		return nil, fmt.Errorf("cred type %q unusable for rightcode: %w", s.CredType, ErrUnsupported)
	}
	tok := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s.Cred.AccessToken), "Bearer "))
	if tok == "" {
		return nil, fmt.Errorf("user_token missing: %w", ErrAuth)
	}
	return &rcSession{hc: s.client(), base: s.BaseURL,
		hdr: map[string]string{"Authorization": "Bearer " + tok}}, nil
}

func (c *rcSession) do(ctx context.Context, method, path string, body, out any) error {
	return doJSON(ctx, c.hc, method, joinURL(c.base, path), c.hdr, body, out)
}

// rcNum accepts 5, "5", "0.40000000", null, "" → *float64 (nil when absent/invalid).
func rcNum(raw json.RawMessage) *float64 {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var f float64
	if json.Unmarshal(raw, &f) == nil {
		return &f
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return nil
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return nil
	}
	return &f
}

// rcTruthy reads a flag that may be a bool, a number (non-zero) or a string ("1"/"true").
func rcTruthy(raw json.RawMessage) bool {
	var b bool
	if json.Unmarshal(raw, &b) == nil {
		return b
	}
	if n := rcNum(raw); n != nil {
		return *n != 0
	}
	return false
}

// Probe: GET /upstreams/public lists the channels; at least one with a "/…" prefix.
func (rightCodeAdapter) Probe(ctx context.Context, s Site) (ProbeResult, error) {
	var out struct {
		Upstreams []struct {
			ID     int    `json:"id"`
			Prefix string `json:"prefix"`
		} `json:"upstreams"`
	}
	if err := getJSON(ctx, s.client(), joinURL(s.BaseURL, "/upstreams/public"), nil, &out); err != nil {
		return ProbeResult{}, fmt.Errorf("probe /upstreams/public: %w", err)
	}
	if len(out.Upstreams) == 0 {
		return ProbeResult{}, errors.New("probe /upstreams/public: no channels")
	}
	for _, u := range out.Upstreams {
		if !strings.HasPrefix(u.Prefix, "/") {
			return ProbeResult{}, errors.New("probe /upstreams/public: channel prefix is not a path")
		}
	}
	return ProbeResult{
		Type:     store.UpstreamRightCode,
		Evidence: fmt.Sprintf("/upstreams/public 返回 %d 个渠道（rightcode）", len(out.Upstreams)),
	}, nil
}

type rcMe struct {
	ID       json.RawMessage `json:"id"`
	IsBanned json.RawMessage `json:"is_banned"`
	Balance  json.RawMessage `json:"balance"`
}

func (c *rcSession) me(ctx context.Context) (rcMe, error) {
	var me rcMe
	if err := c.do(ctx, http.MethodGet, "/auth/me", nil, &me); err != nil {
		return rcMe{}, fmt.Errorf("auth/me: %w", err)
	}
	return me, nil
}

func (rightCodeAdapter) Verify(ctx context.Context, s Site) error {
	c, err := rcOpen(s)
	if err != nil {
		return err
	}
	me, err := c.me(ctx)
	if err != nil {
		return err
	}
	if rcTruthy(me.IsBanned) {
		return fmt.Errorf("账号已封禁: %w", ErrAuth)
	}
	return nil
}

func (rightCodeAdapter) Balance(ctx context.Context, s Site) (Balance, error) {
	c, err := rcOpen(s)
	if err != nil {
		return Balance{}, err
	}
	me, err := c.me(ctx)
	if err != nil {
		return Balance{}, err
	}
	v := rcNum(me.Balance)
	if v == nil {
		return Balance{}, errors.New("auth/me has no balance field")
	}
	return Balance{USD: *v, Raw: "balance=" + strconv.FormatFloat(*v, 'f', -1, 64)}, nil
}

// Announcements reads the public /configs dictionary; public.notice is a JSON string.
func (rightCodeAdapter) Announcements(ctx context.Context, s Site) (Announcement, error) {
	var cfg map[string]string
	if err := getJSON(ctx, s.client(), joinURL(s.BaseURL, "/configs"), nil, &cfg); err != nil {
		return Announcement{}, fmt.Errorf("configs: %w", err)
	}
	raw := strings.TrimSpace(cfg["public.notice"])
	if raw == "" {
		return Announcement{}, nil
	}
	var notice struct {
		Admin []struct {
			Description string `json:"description"`
		} `json:"admin"`
	}
	if err := json.Unmarshal([]byte(raw), &notice); err != nil {
		return Announcement{}, fmt.Errorf("decode public.notice: %w", err)
	}
	parts := make([]string, 0, len(notice.Admin))
	for _, n := range notice.Admin {
		if d := strings.TrimSpace(n.Description); d != "" {
			parts = append(parts, d)
		}
	}
	return Announcement{Content: strings.TrimSpace(strings.Join(parts, "\n\n"))}, nil
}

func (rightCodeAdapter) ListModels(ctx context.Context, s Site, key string) ([]string, error) {
	return listModelsOpenAI(ctx, s, key)
}
