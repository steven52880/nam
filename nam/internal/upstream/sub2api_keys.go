package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
)

// sub2api keys and group rates. Verified from the clansty fork (service/sub2api_client.go):
// GET /api/v1/keys?page=&page_size= → data.items[{key, group{id,name,description,rate_multiplier}}]
// and GET /api/v1/groups/rates → data {"<group id>": float}.
// UNVERIFIED: key fields id, name, status, quota, quota_used and the total counter.

const s2PageSize = 100
const s2MaxPages = 1000

type s2Group struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	RateMultiplier float64 `json:"rate_multiplier"`
}

type s2Key struct {
	ID        int64           `json:"id"`
	Name      string          `json:"name"`
	Key       string          `json:"key"`
	Status    json.RawMessage `json:"status"`
	Quota     float64         `json:"quota"`
	QuotaUsed float64         `json:"quota_used"`
	Group     *s2Group        `json:"group"`
}

// statusCode maps a numeric status or the strings "active"/"enabled" to 1, anything else to 2.
// UNVERIFIED: sub2api's real status representation.
func (k s2Key) statusCode() int {
	var n int
	if json.Unmarshal(k.Status, &n) == nil {
		return n
	}
	var str string
	if json.Unmarshal(k.Status, &str) == nil && (str == "active" || str == "enabled") {
		return 1
	}
	return 2
}

func (k s2Key) toToken() Token {
	t := Token{RemoteID: strconv.FormatInt(k.ID, 10), Name: k.Name, Key: k.Key, Status: k.statusCode()}
	if k.Group != nil {
		t.Group = k.Group.Name
	}
	// UNVERIFIED: quota is a USD limit and 0 means unlimited.
	if k.Quota <= 0 {
		t.Unlimited = true
	} else {
		remain := k.Quota - k.QuotaUsed
		t.QuotaUSD = &remain
	}
	return t
}

func listS2Keys(ctx context.Context, c *s2Session) ([]s2Key, error) {
	var all []s2Key
	for page := 1; page <= s2MaxPages; page++ {
		var pg struct {
			Items []s2Key `json:"items"`
			Total int     `json:"total"`
		}
		path := "/api/v1/keys?page=" + strconv.Itoa(page) + "&page_size=" + strconv.Itoa(s2PageSize)
		if err := c.get(ctx, path, &pg); err != nil {
			return nil, fmt.Errorf("list keys: %w", err)
		}
		all = append(all, pg.Items...)
		if len(pg.Items) < s2PageSize || (pg.Total > 0 && len(all) >= pg.Total) {
			break
		}
	}
	return all, nil
}

func (sub2apiAdapter) ListTokens(ctx context.Context, s Site) ([]Token, error) {
	c, err := s2Open(ctx, s)
	if err != nil {
		return nil, err
	}
	keys, err := listS2Keys(ctx, c)
	if err != nil {
		return nil, err
	}
	out := make([]Token, 0, len(keys))
	for _, k := range keys {
		out = append(out, k.toToken())
	}
	return out, nil
}

// CreateToken: no verified create route exists in the fork, so it is not offered.
func (sub2apiAdapter) CreateToken(context.Context, Site, TokenCreate) (Token, error) {
	return Token{}, fmt.Errorf("sub2api create token: %w", ErrUnsupported)
}

// Pricing exposes group rate multipliers only: the groups come from the user's own keys
// (group.rate_multiplier, overridden by the per-user /api/v1/groups/rates entry, exactly like
// the fork's QueryMetadata). sub2api has no verified per-model price table, so Models is empty.
func (sub2apiAdapter) Pricing(ctx context.Context, s Site) (Pricing, error) {
	c, err := s2Open(ctx, s)
	if err != nil {
		return Pricing{}, err
	}
	keys, err := listS2Keys(ctx, c)
	if err != nil {
		return Pricing{}, err
	}
	var custom map[string]float64
	if err := c.get(ctx, "/api/v1/groups/rates", &custom); err != nil {
		return Pricing{}, fmt.Errorf("group rates: %w", err)
	}
	seen := map[int64]bool{}
	var groups []GroupRatio
	for _, k := range keys {
		g := k.Group
		if g == nil || seen[g.ID] {
			continue
		}
		seen[g.ID] = true
		ratio := g.RateMultiplier
		if r, ok := custom[strconv.FormatInt(g.ID, 10)]; ok {
			ratio = r
		}
		groups = append(groups, GroupRatio{Name: g.Name, Ratio: ratio, Desc: g.Description})
	}
	if len(groups) == 0 {
		return Pricing{}, fmt.Errorf("sub2api pricing: no key groups found: %w", ErrUnsupported)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return Pricing{Groups: groups, Models: []ModelPrice{}}, nil
}
