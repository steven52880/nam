package upstream

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/steven52880/nam/internal/store"
)

func init() { Register(officialAdapter{}) }

// officialAdapter covers direct OpenAI / Anthropic / Google endpoints.
type officialAdapter struct{}

func (officialAdapter) Type() string { return store.UpstreamOfficial }

func (officialAdapter) Probe(context.Context, Site) (ProbeResult, error) {
	return ProbeResult{Type: store.UpstreamOfficial, Evidence: "官方直连，不检测"}, nil
}

func (a officialAdapter) Verify(ctx context.Context, s Site) error {
	_, err := a.ListModels(ctx, s, s.Cred.Key)
	return err
}

func (officialAdapter) Balance(context.Context, Site) (Balance, error) {
	return Balance{}, fmt.Errorf("official balance: %w", ErrUnsupported)
}

func (officialAdapter) ListTokens(_ context.Context, s Site) ([]Token, error) {
	t, err := singleKeyToken(s)
	if err != nil {
		return nil, err
	}
	return []Token{t}, nil
}

func (officialAdapter) CreateToken(context.Context, Site, TokenCreate) (Token, error) {
	return Token{}, fmt.Errorf("official create token: %w", ErrUnsupported)
}

func (officialAdapter) Pricing(context.Context, Site) (Pricing, error) {
	return Pricing{}, fmt.Errorf("official pricing: %w", ErrUnsupported)
}

func (officialAdapter) Announcements(context.Context, Site) (Announcement, error) {
	return Announcement{}, fmt.Errorf("official announcements: %w", ErrUnsupported)
}

func (officialAdapter) ListModels(ctx context.Context, s Site, key string) ([]string, error) {
	switch {
	case strings.Contains(s.BaseURL, "anthropic.com"):
		return listModelsAnthropic(ctx, s, key)
	case strings.Contains(s.BaseURL, "generativelanguage.googleapis.com"):
		return listModelsGoogle(ctx, s, key)
	default:
		return listModelsOpenAI(ctx, s, key)
	}
}

func listModelsAnthropic(ctx context.Context, s Site, key string) ([]string, error) {
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	hdr := map[string]string{"x-api-key": key, "anthropic-version": "2023-06-01"}
	if err := getJSON(ctx, s.client(), joinURL(s.BaseURL, "/v1/models?limit=1000"), hdr, &out); err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	ids := make([]string, 0, len(out.Data))
	for _, m := range out.Data {
		ids = append(ids, m.ID)
	}
	return sortedUnique(ids), nil
}

func listModelsGoogle(ctx context.Context, s Site, key string) ([]string, error) {
	var out struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	u := joinURL(s.BaseURL, "/v1beta/models?pageSize=1000&key="+url.QueryEscape(key))
	if err := getJSON(ctx, s.client(), u, nil, &out); err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	ids := make([]string, 0, len(out.Models))
	for _, m := range out.Models {
		ids = append(ids, strings.TrimPrefix(m.Name, "models/"))
	}
	return sortedUnique(ids), nil
}
