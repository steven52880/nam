// Package upstream defines the adapter contract for upstream sites and a registry.
// Implementations live in subpackages-free files of this package (newapi.go, skkey.go, ...).
package upstream

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"
)

// ErrUnsupported means the adapter cannot do this operation for this upstream
// (e.g. sk-key upstream has no token list). Callers treat it as "manual".
var ErrUnsupported = errors.New("upstream: operation not supported")

// ErrAuth means credentials were rejected.
var ErrAuth = errors.New("upstream: credentials rejected")

// Credential is the decrypted credential of an upstream (same JSON as api.Credential).
type Credential struct {
	AccessToken string `json:"access_token,omitempty"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	Key         string `json:"key,omitempty"`
}

// Site is everything an adapter needs to talk to one upstream.
type Site struct {
	ID       uint
	Name     string
	Type     string // store.Upstream* constants
	BaseURL  string // no trailing slash, no /v1
	CredType string // store.Cred* constants
	Cred     Credential
	UserID   int    // platform user id (NewAPI New-Api-User)
	Config   string // adapter_config JSON (generic)
	// HTTP is the client to use (shared, with timeouts). Nil → adapter default.
	HTTP *http.Client
}

// ProbeResult is what auto-detection found.
type ProbeResult struct {
	Type     string `json:"type"`
	Evidence string `json:"evidence"` // human-readable reason, e.g. "/api/status 返回 quota_per_unit"
	Version  string `json:"version,omitempty"`
}

// Balance is the remaining credit in upstream USD (quota / quota_per_unit).
type Balance struct {
	USD       float64 `json:"usd"`
	Unlimited bool    `json:"unlimited"`
	Raw       string  `json:"raw,omitempty"` // short raw excerpt for debugging
}

// Token is one API key on the upstream.
type Token struct {
	RemoteID  string   `json:"remote_id"`
	Name      string   `json:"name"`
	Group     string   `json:"group"`
	Key       string   `json:"key"` // full key; may be empty if the upstream never reveals it
	QuotaUSD  *float64 `json:"quota_usd"`
	Unlimited bool     `json:"unlimited"`
	Status    int      `json:"status"` // 1 = enabled
}

// TokenCreate is the request for creating a token on the upstream.
type TokenCreate struct {
	Name     string   `json:"name"`
	Group    string   `json:"group"`
	QuotaUSD *float64 `json:"quota_usd"` // nil = unlimited
}

// GroupRatio is one upstream group with its ratio.
type GroupRatio struct {
	Name  string  `json:"name"`
	Ratio float64 `json:"ratio"`
	Desc  string  `json:"desc"`
}

// ModelPrice is one model's price on the upstream, already in upstream USD per 1M tokens
// (or per request) for the given group.
type ModelPrice struct {
	Model      string   `json:"model"`
	Group      string   `json:"group"`
	ModelRatio *float64 `json:"model_ratio"`
	Input      *float64 `json:"input"`
	Output     *float64 `json:"output"`
	CacheRead  *float64 `json:"cache_read"`
	CacheWrite *float64 `json:"cache_write"`
	PerRequest *float64 `json:"per_request"`
}

// Pricing is the full pricing snapshot of an upstream.
type Pricing struct {
	Groups []GroupRatio `json:"groups"`
	Models []ModelPrice `json:"models"`
}

// Announcement is the upstream's notice text.
type Announcement struct {
	Content string `json:"content"`
}

// Adapter talks to one kind of upstream. Every method must honour ctx and must return
// ErrUnsupported (possibly wrapped) when the operation does not exist for this kind.
type Adapter interface {
	// Type is the store.Upstream* constant this adapter handles.
	Type() string
	// Probe checks reachability and returns detected type info. It must not need credentials.
	Probe(ctx context.Context, s Site) (ProbeResult, error)
	// Verify checks the credentials (cheap call). Returns ErrAuth on rejection.
	Verify(ctx context.Context, s Site) error
	Balance(ctx context.Context, s Site) (Balance, error)
	ListTokens(ctx context.Context, s Site) ([]Token, error)
	CreateToken(ctx context.Context, s Site, t TokenCreate) (Token, error)
	Pricing(ctx context.Context, s Site) (Pricing, error)
	Announcements(ctx context.Context, s Site) (Announcement, error)
	// ListModels calls /v1/models with an API key (token key), returns model ids.
	ListModels(ctx context.Context, s Site, key string) ([]string, error)
}

var (
	regMu sync.RWMutex
	reg   = map[string]Adapter{}
)

// Register adds an adapter. Called from init() of each adapter file.
func Register(a Adapter) {
	regMu.Lock()
	defer regMu.Unlock()
	if _, dup := reg[a.Type()]; dup {
		panic(fmt.Sprintf("upstream: duplicate adapter %q", a.Type()))
	}
	reg[a.Type()] = a
}

// Get returns the adapter for a type.
func Get(typ string) (Adapter, error) {
	regMu.RLock()
	defer regMu.RUnlock()
	a, ok := reg[typ]
	if !ok {
		return nil, fmt.Errorf("upstream: no adapter for type %q", typ)
	}
	return a, nil
}

// Types lists registered adapter types (sorted).
func Types() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(reg))
	for t := range reg {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}
