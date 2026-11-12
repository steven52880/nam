package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// rcUpstream is one Right Code channel (= one project group; its prefix is the group name).
type rcUpstream struct {
	ID     int             `json:"id"`
	Name   string          `json:"name"`
	Prefix string          `json:"prefix"`
	Rate   json.RawMessage `json:"rate"`
}

// rcUpstreams reads /upstreams/effective with the session; on a non-auth failure it falls
// back to the anonymous /upstreams/public.
func rcUpstreams(ctx context.Context, s Site) ([]rcUpstream, error) {
	c, err := rcOpen(s)
	if err != nil {
		return nil, err
	}
	var out struct {
		Upstreams []rcUpstream `json:"upstreams"`
	}
	err = c.do(ctx, http.MethodGet, "/upstreams/effective", nil, &out)
	if err != nil {
		if errors.Is(err, ErrAuth) {
			return nil, fmt.Errorf("upstreams/effective: %w", err)
		}
		out.Upstreams = nil
		if perr := getJSON(ctx, s.client(), joinURL(s.BaseURL, "/upstreams/public"), nil, &out); perr != nil {
			return nil, fmt.Errorf("upstreams/public: %w", perr)
		}
	}
	return out.Upstreams, nil
}

type rcKey struct {
	ID                       int             `json:"id"`
	BoundUpstreamID          *int            `json:"bound_upstream_id"`
	Key                      string          `json:"key"`
	Name                     string          `json:"name"`
	QuotaLimit               json.RawMessage `json:"quota_limit"`
	UsedQuota                json.RawMessage `json:"used_quota"`
	ExpiredAt                *string         `json:"expired_at"`
	IsActive                 bool            `json:"is_active"`
	DisabledBySecurityReview *bool           `json:"disabled_by_security_review"`
}

// rcCST is UTC+8; expired_at has no zone and is Beijing time. Fixed zone: no tzdata needed.
var rcCST = time.FixedZone("CST", 8*3600)

func (k rcKey) expired(now time.Time) bool {
	if k.ExpiredAt == nil || *k.ExpiredAt == "" {
		return false
	}
	t, err := time.ParseInLocation("2006-01-02T15:04:05", *k.ExpiredAt, rcCST)
	return err == nil && t.Before(now)
}

func (k rcKey) toToken(prefixByID map[int]string, now time.Time) Token {
	t := Token{RemoteID: strconv.Itoa(k.ID), Name: k.Name, Key: k.Key, Status: 2}
	if k.BoundUpstreamID != nil {
		id := *k.BoundUpstreamID
		if p, ok := prefixByID[id]; ok {
			t.Group = p
		} else {
			t.Group = "#" + strconv.Itoa(id)
		}
	}
	disabled := k.DisabledBySecurityReview != nil && *k.DisabledBySecurityReview
	if k.IsActive && !disabled && !k.expired(now) {
		t.Status = 1
	}
	if limit := rcNum(k.QuotaLimit); limit == nil {
		t.Unlimited = true
	} else {
		remain := *limit
		if used := rcNum(k.UsedQuota); used != nil {
			remain -= *used
		}
		if remain < 0 {
			remain = 0
		}
		t.QuotaUSD = &remain
	}
	return t
}

func rcPrefixMap(ups []rcUpstream) map[int]string {
	m := make(map[int]string, len(ups))
	for _, u := range ups {
		m[u.ID] = u.Prefix
	}
	return m
}

func (c *rcSession) listKeys(ctx context.Context) ([]rcKey, error) {
	var out struct {
		Keys []rcKey `json:"keys"`
	}
	if err := c.do(ctx, http.MethodGet, "/api-key/list", nil, &out); err != nil {
		return nil, fmt.Errorf("list keys: %w", err)
	}
	return out.Keys, nil
}

func (rightCodeAdapter) ListTokens(ctx context.Context, s Site) ([]Token, error) {
	c, err := rcOpen(s)
	if err != nil {
		return nil, err
	}
	keys, err := c.listKeys(ctx)
	if err != nil {
		return nil, err
	}
	ups, err := rcUpstreams(ctx, s)
	if err != nil {
		return nil, err
	}
	byID, now := rcPrefixMap(ups), time.Now()
	out := make([]Token, 0, len(keys))
	for _, k := range keys {
		out = append(out, k.toToken(byID, now))
	}
	return out, nil
}

// CreateToken only creates channel-bound keys; legacy unbound keys are not supported.
func (rightCodeAdapter) CreateToken(ctx context.Context, s Site, in TokenCreate) (Token, error) {
	c, err := rcOpen(s)
	if err != nil {
		return Token{}, err
	}
	ups, err := rcUpstreams(ctx, s)
	if err != nil {
		return Token{}, err
	}
	chanID := 0
	for _, u := range ups {
		if in.Group != "" && u.Prefix == in.Group {
			chanID = u.ID
			break
		}
	}
	if chanID == 0 {
		return Token{}, fmt.Errorf("rightcode: 未知分组 %q", in.Group)
	}
	body := map[string]any{
		"name":              in.Name,
		"bound_upstream_id": chanID,
		"allowed_models":    []string{},
		"allow_wallet":      true,
		"allowed_item_ids":  []int{},
		"quota_limit":       in.QuotaUSD, // nil → JSON null (unlimited)
	}
	var created struct {
		ID int `json:"id"`
	}
	if err := c.do(ctx, http.MethodPost, "/api-key/create", body, &created); err != nil {
		return Token{}, fmt.Errorf("create key: %w", err)
	}
	byID, now := rcPrefixMap(ups), time.Now()
	if created.ID > 0 {
		var k rcKey
		if err := c.do(ctx, http.MethodGet, "/api-key/"+strconv.Itoa(created.ID), nil, &k); err != nil {
			return Token{}, fmt.Errorf("get created key: %w", err)
		}
		return k.toToken(byID, now), nil
	}
	keys, err := c.listKeys(ctx)
	if err != nil {
		return Token{}, err
	}
	var best *rcKey
	for i := range keys {
		if keys[i].Name == in.Name && (best == nil || keys[i].ID > best.ID) {
			best = &keys[i]
		}
	}
	if best == nil {
		return Token{}, errors.New("create key: created key not found in list")
	}
	return best.toToken(byID, now), nil
}
