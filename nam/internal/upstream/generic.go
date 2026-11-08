package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/steven52880/nam/internal/store"
)

const maxAnnouncementBytes = 64 << 10

func init() { Register(genericAdapter{}) }

type genericBalanceCfg struct {
	URL      string            `json:"url"`
	Method   string            `json:"method"`
	Headers  map[string]string `json:"headers"`
	JSONPath string            `json:"json_path"`
	Factor   *float64          `json:"factor"`
}

type genericCfg struct {
	Balance         *genericBalanceCfg `json:"balance"`
	AnnouncementURL string             `json:"announcement_url"`
}

func parseGenericConfig(raw string) (genericCfg, error) {
	var c genericCfg
	if strings.TrimSpace(raw) == "" {
		return c, nil
	}
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return c, fmt.Errorf("generic config: invalid JSON: %w", err)
	}
	return c, nil
}

// ValidateGenericConfig checks the adapter_config of a generic upstream.
// An empty config is valid (no balance, no announcement).
func ValidateGenericConfig(raw string) error {
	c, err := parseGenericConfig(raw)
	if err != nil {
		return err
	}
	b := c.Balance
	if b == nil {
		return nil
	}
	if strings.TrimSpace(b.URL) == "" {
		return errors.New("generic config: balance.url is required")
	}
	if strings.TrimSpace(b.JSONPath) == "" {
		return errors.New("generic config: balance.json_path is required")
	}
	if b.Factor != nil && (math.IsNaN(*b.Factor) || math.IsInf(*b.Factor, 0)) {
		return errors.New("generic config: balance.factor must be a finite number")
	}
	if m := strings.ToUpper(b.Method); m != "" && m != http.MethodGet && m != http.MethodPost {
		return errors.New("generic config: balance.method must be GET or POST")
	}
	return nil
}

// resolveURL accepts absolute URLs or paths relative to the site base.
func resolveURL(base, u string) string {
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	if !strings.HasPrefix(u, "/") {
		u = "/" + u
	}
	return joinURL(base, u)
}

// jsonPathNumber walks a dot path (array indices allowed) and returns the number found.
func jsonPathNumber(root any, path string) (float64, error) {
	cur := root
	for _, seg := range strings.Split(path, ".") {
		switch v := cur.(type) {
		case map[string]any:
			next, ok := v[seg]
			if !ok {
				return 0, fmt.Errorf("json_path %q: key %q not found", path, seg)
			}
			cur = next
		case []any:
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(v) {
				return 0, fmt.Errorf("json_path %q: bad index %q", path, seg)
			}
			cur = v[i]
		default:
			return 0, fmt.Errorf("json_path %q: cannot descend into %q", path, seg)
		}
	}
	switch n := cur.(type) {
	case float64:
		return n, nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err != nil {
			return 0, fmt.Errorf("json_path %q: value is not numeric", path)
		}
		return f, nil
	default:
		return 0, fmt.Errorf("json_path %q: value is not a number", path)
	}
}

func substitute(v string, c Credential) string {
	v = strings.ReplaceAll(v, "{{key}}", c.Key)
	return strings.ReplaceAll(v, "{{token}}", c.AccessToken)
}

type genericAdapter struct{}

func (genericAdapter) Type() string { return store.UpstreamGeneric }

func (genericAdapter) Probe(ctx context.Context, s Site) (ProbeResult, error) {
	status, _, err := doRaw(ctx, s.client(), http.MethodGet, s.BaseURL, nil, nil)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("probe: %w", err)
	}
	return ProbeResult{Type: store.UpstreamGeneric, Evidence: fmt.Sprintf("可访问（HTTP %d）", status)}, nil
}

func (a genericAdapter) Verify(ctx context.Context, s Site) error {
	cfg, err := parseGenericConfig(s.Config)
	if err != nil {
		return err
	}
	if cfg.Balance != nil {
		_, err = a.Balance(ctx, s)
		return err
	}
	if s.Cred.Key != "" {
		_, err = a.ListModels(ctx, s, s.Cred.Key)
		return err
	}
	return fmt.Errorf("generic verify: %w", ErrUnsupported)
}

func (genericAdapter) Balance(ctx context.Context, s Site) (Balance, error) {
	if err := ValidateGenericConfig(s.Config); err != nil {
		return Balance{}, err
	}
	cfg, _ := parseGenericConfig(s.Config)
	b := cfg.Balance
	if b == nil {
		return Balance{}, fmt.Errorf("generic balance not configured: %w", ErrUnsupported)
	}
	hdr := make(map[string]string, len(b.Headers))
	for k, v := range b.Headers {
		hdr[k] = substitute(v, s.Cred)
	}
	method := strings.ToUpper(b.Method)
	if method == "" {
		method = http.MethodGet
	}
	var root any
	if err := doJSON(ctx, s.client(), method, resolveURL(s.BaseURL, b.URL), hdr, nil, &root); err != nil {
		return Balance{}, fmt.Errorf("generic balance: %w", err)
	}
	n, err := jsonPathNumber(root, b.JSONPath)
	if err != nil {
		return Balance{}, err
	}
	factor := 1.0
	if b.Factor != nil {
		factor = *b.Factor
	}
	return Balance{USD: n * factor, Raw: fmt.Sprintf("%s=%g factor=%g", b.JSONPath, n, factor)}, nil
}

func (genericAdapter) ListTokens(_ context.Context, s Site) ([]Token, error) {
	t, err := singleKeyToken(s)
	if err != nil {
		return nil, err
	}
	return []Token{t}, nil
}

func (genericAdapter) CreateToken(context.Context, Site, TokenCreate) (Token, error) {
	return Token{}, fmt.Errorf("generic create token: %w", ErrUnsupported)
}

func (genericAdapter) Pricing(context.Context, Site) (Pricing, error) {
	return Pricing{}, fmt.Errorf("generic pricing: %w", ErrUnsupported)
}

func (genericAdapter) Announcements(ctx context.Context, s Site) (Announcement, error) {
	cfg, err := parseGenericConfig(s.Config)
	if err != nil {
		return Announcement{}, err
	}
	if cfg.AnnouncementURL == "" {
		return Announcement{}, fmt.Errorf("generic announcements not configured: %w", ErrUnsupported)
	}
	status, body, err := doRaw(ctx, s.client(), http.MethodGet, resolveURL(s.BaseURL, cfg.AnnouncementURL), nil, nil)
	if err != nil {
		return Announcement{}, fmt.Errorf("generic announcements: %w", err)
	}
	if status < 200 || status >= 300 {
		return Announcement{}, fmt.Errorf("generic announcements: HTTP %d", status)
	}
	if len(body) > maxAnnouncementBytes {
		body = body[:maxAnnouncementBytes]
	}
	return Announcement{Content: strings.TrimSpace(string(body))}, nil
}

func (genericAdapter) ListModels(ctx context.Context, s Site, key string) ([]string, error) {
	return listModelsOpenAI(ctx, s, key)
}
