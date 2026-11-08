package upstream

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/steven52880/nam/internal/store"
)

const defaultQuotaPerUnit = 500000

func init() { Register(newAPIAdapter{}) }

// newAPIAdapter covers NewAPI / VoAPI / Veloera style panels.
type newAPIAdapter struct{}

func (newAPIAdapter) Type() string { return store.UpstreamNewAPI }

// naSession carries the auth headers (and cookie jar client) for one adapter call.
type naSession struct {
	hc   *http.Client
	base string
	hdr  map[string]string
}

func (c *naSession) get(ctx context.Context, path string, out any) error {
	return callEnvelope(ctx, c.hc, http.MethodGet, joinURL(c.base, path), c.hdr, nil, out)
}

func (c *naSession) post(ctx context.Context, path string, body, out any) error {
	return callEnvelope(ctx, c.hc, http.MethodPost, joinURL(c.base, path), c.hdr, body, out)
}

// newSession authenticates once per adapter call. Password creds log in with a
// per-call cookie jar; access tokens are sent raw (NewAPI style).
func newSession(ctx context.Context, s Site) (*naSession, error) {
	c := &naSession{hc: s.client(), base: s.BaseURL, hdr: map[string]string{}}
	userID := s.UserID
	switch s.CredType {
	case store.CredPassword:
		jar, err := cookiejar.New(nil)
		if err != nil {
			return nil, fmt.Errorf("cookie jar: %w", err)
		}
		hc := *s.client()
		hc.Jar = jar
		c.hc = &hc
		var me struct {
			ID int `json:"id"`
		}
		body := map[string]string{"username": s.Cred.Username, "password": s.Cred.Password}
		if err := c.post(ctx, "/api/user/login", body, &me); err != nil {
			if apiErr, ok := err.(*apiError); ok { //nolint:errorlint // sentinel type from our own helper
				return nil, fmt.Errorf("login failed: %s: %w", apiErr.msg, ErrAuth)
			}
			return nil, fmt.Errorf("login: %w", err)
		}
		if userID == 0 {
			userID = me.ID
		}
	case store.CredAccessToken:
		if s.Cred.AccessToken == "" {
			return nil, fmt.Errorf("access token missing: %w", ErrAuth)
		}
		c.hdr["Authorization"] = s.Cred.AccessToken
	default:
		return nil, fmt.Errorf("cred type %q unusable for newapi: %w", s.CredType, ErrUnsupported)
	}
	if userID > 0 {
		c.hdr["New-Api-User"] = strconv.Itoa(userID)
	}
	return c, nil
}

type statusData struct {
	QuotaPerUnit float64 `json:"quota_per_unit"`
	Version      string  `json:"version"`
}

func fetchStatus(ctx context.Context, s Site) (statusData, error) {
	var d statusData
	err := callEnvelope(ctx, s.client(), http.MethodGet, joinURL(s.BaseURL, "/api/status"), nil, nil, &d)
	return d, err
}

func quotaPerUnit(ctx context.Context, s Site) (float64, error) {
	d, err := fetchStatus(ctx, s)
	if err != nil {
		return 0, fmt.Errorf("status: %w", err)
	}
	if d.QuotaPerUnit <= 0 {
		return defaultQuotaPerUnit, nil
	}
	return d.QuotaPerUnit, nil
}

// probeNewAPILike detects NewAPI-family sites through /api/status.
func probeNewAPILike(ctx context.Context, s Site, typ, label string) (ProbeResult, error) {
	d, err := fetchStatus(ctx, s)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("probe /api/status: %w", err)
	}
	if d.QuotaPerUnit <= 0 {
		return ProbeResult{}, fmt.Errorf("probe: /api/status has no quota_per_unit, not a NewAPI-family site")
	}
	return ProbeResult{
		Type:     typ,
		Evidence: fmt.Sprintf("/api/status 返回 quota_per_unit=%s（%s）", strconv.FormatFloat(d.QuotaPerUnit, 'f', -1, 64), label),
		Version:  d.Version,
	}, nil
}

func (newAPIAdapter) Probe(ctx context.Context, s Site) (ProbeResult, error) {
	return probeNewAPILike(ctx, s, store.UpstreamNewAPI, "newapi")
}

type selfData struct {
	ID    int   `json:"id"`
	Quota int64 `json:"quota"`
}

func (newAPIAdapter) Verify(ctx context.Context, s Site) error {
	c, err := newSession(ctx, s)
	if err != nil {
		return err
	}
	return c.get(ctx, "/api/user/self", nil)
}

func (newAPIAdapter) Balance(ctx context.Context, s Site) (Balance, error) {
	qpu, err := quotaPerUnit(ctx, s)
	if err != nil {
		return Balance{}, err
	}
	c, err := newSession(ctx, s)
	if err != nil {
		return Balance{}, err
	}
	var me selfData
	if err := c.get(ctx, "/api/user/self", &me); err != nil {
		return Balance{}, fmt.Errorf("user/self: %w", err)
	}
	return Balance{USD: float64(me.Quota) / qpu, Raw: fmt.Sprintf("quota=%d quota_per_unit=%g", me.Quota, qpu)}, nil
}

type remoteToken struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Key            string `json:"key"`
	Status         int    `json:"status"`
	RemainQuota    int64  `json:"remain_quota"`
	UnlimitedQuota bool   `json:"unlimited_quota"`
	Group          string `json:"group"`
}

func (t remoteToken) toToken(qpu float64) Token {
	out := Token{RemoteID: strconv.Itoa(t.ID), Name: t.Name, Group: t.Group, Key: t.Key, Unlimited: t.UnlimitedQuota, Status: t.Status}
	if !t.UnlimitedQuota {
		usd := float64(t.RemainQuota) / qpu
		out.QuotaUSD = &usd
	}
	return out
}

func listRemoteTokens(ctx context.Context, c *naSession) ([]remoteToken, error) {
	const pageSize = 100
	var all []remoteToken
	for page := 1; ; page++ {
		var pg struct {
			Items []remoteToken `json:"items"`
			Total int           `json:"total"`
		}
		path := "/api/token/?p=" + strconv.Itoa(page) + "&page_size=" + strconv.Itoa(pageSize)
		if err := c.get(ctx, path, &pg); err != nil {
			return nil, fmt.Errorf("list tokens: %w", err)
		}
		all = append(all, pg.Items...)
		if len(pg.Items) == 0 || len(all) >= pg.Total {
			return all, nil
		}
	}
}

// fillKey replaces a masked list key with the real one via POST /api/token/:id/key.
// NewAPI stores keys without the "sk-" prefix; it is added so the key is usable as-is.
func fillKey(ctx context.Context, c *naSession, t *Token) error {
	var k struct {
		Key string `json:"key"`
	}
	if err := c.post(ctx, "/api/token/"+url.PathEscape(t.RemoteID)+"/key", nil, &k); err != nil {
		if errIsAuth(err) {
			return err
		}
		if strings.Contains(t.Key, "*") {
			t.Key = ""
		}
		return nil
	}
	if k.Key != "" && !strings.HasPrefix(k.Key, "sk-") {
		k.Key = "sk-" + k.Key
	}
	t.Key = k.Key
	return nil
}

func (newAPIAdapter) ListTokens(ctx context.Context, s Site) ([]Token, error) {
	qpu, err := quotaPerUnit(ctx, s)
	if err != nil {
		return nil, err
	}
	c, err := newSession(ctx, s)
	if err != nil {
		return nil, err
	}
	rts, err := listRemoteTokens(ctx, c)
	if err != nil {
		return nil, err
	}
	out := make([]Token, 0, len(rts))
	for _, rt := range rts {
		t := rt.toToken(qpu)
		if err := fillKey(ctx, c, &t); err != nil {
			return nil, fmt.Errorf("token %s key: %w", t.RemoteID, err)
		}
		out = append(out, t)
	}
	return out, nil
}

func (newAPIAdapter) CreateToken(ctx context.Context, s Site, in TokenCreate) (Token, error) {
	qpu, err := quotaPerUnit(ctx, s)
	if err != nil {
		return Token{}, err
	}
	c, err := newSession(ctx, s)
	if err != nil {
		return Token{}, err
	}
	body := map[string]any{"name": in.Name, "group": in.Group, "expired_time": -1, "unlimited_quota": in.QuotaUSD == nil}
	if in.QuotaUSD != nil {
		body["remain_quota"] = int64(*in.QuotaUSD*qpu + 0.5)
	}
	if err := c.post(ctx, "/api/token/", body, nil); err != nil {
		return Token{}, fmt.Errorf("create token: %w", err)
	}
	rts, err := listRemoteTokens(ctx, c)
	if err != nil {
		return Token{}, err
	}
	sort.Slice(rts, func(i, j int) bool { return rts[i].ID > rts[j].ID })
	for _, rt := range rts {
		if rt.Name != in.Name {
			continue
		}
		t := rt.toToken(qpu)
		if err := fillKey(ctx, c, &t); err != nil {
			return Token{}, fmt.Errorf("token %s key: %w", t.RemoteID, err)
		}
		return t, nil
	}
	return Token{}, fmt.Errorf("created token %q not found in token list", in.Name)
}

func (newAPIAdapter) ListModels(ctx context.Context, s Site, key string) ([]string, error) {
	return listModelsOpenAI(ctx, s, key)
}
