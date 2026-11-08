package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"sort"
	"strconv"
	"strings"

	"github.com/steven52880/nam/internal/store"
)

// OneHub / DoneHub adapter.
//
// Sources (github.com/MartialBE/one-hub, branch main, fetched 2026-11-08):
//   - router/api-router.go: route table and which middleware guards each route.
//   - middleware/auth.go authHelper: session cookie, or raw `Authorization: <access token>`
//     (model/user.go ValidateAccessToken strips an optional "Bearer "). No New-Api-User header.
//     A rejected access token answers HTTP 200 {success:false,"无权进行此操作，access token 无效"}.
//   - controller/user.go Login: POST /api/user/login {username,password} sets a session cookie.
//   - controller/misc.go GetStatus (quota_per_unit, OneHub-only keys) and GetNotice (data = string).
//   - controller/token.go + model/token.go + model/common.go: GET /api/token/?page=&size= returns
//     data={data:[Token],page,size,total_count}; size is capped at 100 (config.MaxRecentItems).
//     Keys are returned unmasked; AddToken only answers {success}.

const (
	ohPageSize = 100
	ohMaxPages = 1000
)

func init() { Register(oneHubAdapter{}) }

type oneHubAdapter struct{}

func (oneHubAdapter) Type() string { return store.UpstreamOneHub }

// ohStatusMarkers are /api/status keys that GetStatus emits only in OneHub
// (controller/misc.go). UNVERIFIED: that NewAPI never emits any of them.
var ohStatusMarkers = []string{"PaymentUSDRate", "RechargeDiscount", "UptimeEnabled", "SafeToolName"}

func (oneHubAdapter) Probe(ctx context.Context, s Site) (ProbeResult, error) {
	var d map[string]json.RawMessage
	err := callEnvelope(ctx, s.client(), http.MethodGet, joinURL(s.BaseURL, "/api/status"), nil, nil, &d)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("probe /api/status: %w", err)
	}
	var qpu float64
	if json.Unmarshal(d["quota_per_unit"], &qpu) != nil || qpu <= 0 {
		return ProbeResult{}, errors.New("probe: /api/status has no quota_per_unit, not a OneHub site")
	}
	for _, k := range ohStatusMarkers {
		if _, ok := d[k]; !ok {
			continue
		}
		var version string
		_ = json.Unmarshal(d["version"], &version)
		return ProbeResult{
			Type:     store.UpstreamOneHub,
			Evidence: fmt.Sprintf("/api/status 返回 quota_per_unit=%g 且含 OneHub 特有字段 %s", qpu, k),
			Version:  version,
		}, nil
	}
	return ProbeResult{}, errors.New("probe: /api/status lacks OneHub-specific fields")
}

// ohSession authenticates once per adapter call: password creds log in with a
// per-call cookie jar, access tokens are sent raw in Authorization.
func ohSession(ctx context.Context, s Site) (*naSession, error) {
	c := &naSession{hc: s.client(), base: s.BaseURL, hdr: map[string]string{}}
	switch s.CredType {
	case store.CredPassword:
		jar, err := cookiejar.New(nil)
		if err != nil {
			return nil, fmt.Errorf("cookie jar: %w", err)
		}
		hc := *s.client()
		hc.Jar = jar
		c.hc = &hc
		body := map[string]string{"username": s.Cred.Username, "password": s.Cred.Password}
		if err := c.post(ctx, "/api/user/login", body, nil); err != nil {
			var ae *apiError
			if errors.As(err, &ae) {
				return nil, fmt.Errorf("login failed: %s: %w", ae.msg, ErrAuth)
			}
			return nil, fmt.Errorf("login: %w", err)
		}
	case store.CredAccessToken:
		if s.Cred.AccessToken == "" {
			return nil, fmt.Errorf("access token missing: %w", ErrAuth)
		}
		c.hdr["Authorization"] = s.Cred.AccessToken
	default:
		return nil, fmt.Errorf("cred type %q unusable for onehub: %w", s.CredType, ErrUnsupported)
	}
	return c, nil
}

func (oneHubAdapter) Verify(ctx context.Context, s Site) error {
	c, err := ohSession(ctx, s)
	if err != nil {
		return err
	}
	return c.get(ctx, "/api/user/self", nil)
}

func (oneHubAdapter) Balance(ctx context.Context, s Site) (Balance, error) {
	qpu, err := quotaPerUnit(ctx, s)
	if err != nil {
		return Balance{}, err
	}
	c, err := ohSession(ctx, s)
	if err != nil {
		return Balance{}, err
	}
	// model.User: quota (remaining, int), used_quota. 1 USD = quota_per_unit quota.
	var me selfData
	if err := c.get(ctx, "/api/user/self", &me); err != nil {
		return Balance{}, fmt.Errorf("user/self: %w", err)
	}
	return Balance{USD: float64(me.Quota) / qpu, Raw: fmt.Sprintf("quota=%d quota_per_unit=%g", me.Quota, qpu)}, nil
}

// ohToken mirrors model.Token (model/token.go).
type ohToken struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Key            string `json:"key"`
	Status         int    `json:"status"`
	RemainQuota    int64  `json:"remain_quota"`
	UnlimitedQuota bool   `json:"unlimited_quota"`
	Group          string `json:"group"`
}

func (t ohToken) toToken(qpu float64) Token {
	key := t.Key
	if strings.Contains(key, "*") {
		key = ""
	} else if key != "" && !strings.HasPrefix(key, "sk-") {
		// UNVERIFIED: whether common.GenerateToken stores the "sk-" prefix. tokenAuth accepts
		// the key with or without it, so the prefixed form is always safe.
		key = "sk-" + key
	}
	out := Token{RemoteID: strconv.Itoa(t.ID), Name: t.Name, Group: t.Group, Key: key, Unlimited: t.UnlimitedQuota, Status: t.Status}
	if !t.UnlimitedQuota {
		usd := float64(t.RemainQuota) / qpu
		out.QuotaUSD = &usd
	}
	return out
}

func listOHTokens(ctx context.Context, c *naSession) ([]ohToken, error) {
	var all []ohToken
	for page := 1; page <= ohMaxPages; page++ {
		var pg struct {
			Data       []ohToken `json:"data"`
			TotalCount int       `json:"total_count"`
		}
		path := "/api/token/?page=" + strconv.Itoa(page) + "&size=" + strconv.Itoa(ohPageSize)
		if err := c.get(ctx, path, &pg); err != nil {
			return nil, fmt.Errorf("list tokens: %w", err)
		}
		all = append(all, pg.Data...)
		if len(pg.Data) == 0 || len(all) >= pg.TotalCount {
			break
		}
	}
	return all, nil
}

func (oneHubAdapter) ListTokens(ctx context.Context, s Site) ([]Token, error) {
	qpu, err := quotaPerUnit(ctx, s)
	if err != nil {
		return nil, err
	}
	c, err := ohSession(ctx, s)
	if err != nil {
		return nil, err
	}
	ts, err := listOHTokens(ctx, c)
	if err != nil {
		return nil, err
	}
	out := make([]Token, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.toToken(qpu))
	}
	return out, nil
}

func (oneHubAdapter) CreateToken(ctx context.Context, s Site, in TokenCreate) (Token, error) {
	qpu, err := quotaPerUnit(ctx, s)
	if err != nil {
		return Token{}, err
	}
	c, err := ohSession(ctx, s)
	if err != nil {
		return Token{}, err
	}
	// controller/token.go AddToken copies name, group, expired_time (-1 = never),
	// remain_quota, unlimited_quota; the key is generated server-side in AfterCreate.
	body := map[string]any{"name": in.Name, "group": in.Group, "expired_time": -1, "unlimited_quota": in.QuotaUSD == nil}
	if in.QuotaUSD != nil {
		body["remain_quota"] = int64(*in.QuotaUSD*qpu + 0.5)
	}
	if err := c.post(ctx, "/api/token/", body, nil); err != nil {
		return Token{}, fmt.Errorf("create token: %w", err)
	}
	ts, err := listOHTokens(ctx, c)
	if err != nil {
		return Token{}, err
	}
	sort.Slice(ts, func(i, j int) bool { return ts[i].ID > ts[j].ID })
	for _, t := range ts {
		if t.Name == in.Name {
			return t.toToken(qpu), nil
		}
	}
	return Token{}, fmt.Errorf("created token %q not found in token list", in.Name)
}

// Announcements: GetNotice has no auth middleware and returns data = string.
func (oneHubAdapter) Announcements(ctx context.Context, s Site) (Announcement, error) {
	c := &naSession{hc: s.client(), base: s.BaseURL, hdr: map[string]string{}}
	var text string
	if err := c.get(ctx, "/api/notice", &text); err != nil {
		return Announcement{}, fmt.Errorf("notice: %w", err)
	}
	return Announcement{Content: strings.TrimSpace(text)}, nil
}

func (oneHubAdapter) ListModels(ctx context.Context, s Site, key string) ([]string, error) {
	return listModelsOpenAI(ctx, s, key)
}
