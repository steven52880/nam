package upstream

import (
	"context"
	"fmt"

	"github.com/steven52880/nam/internal/store"
)

// unlimitedThresholdUSD: NewAPI controller/billing.go reports unlimited tokens as 100000000.
const unlimitedThresholdUSD = 100000000

func init() { Register(skKeyAdapter{}) }

// skKeyAdapter handles NewAPI-like sites where only an API key is available.
type skKeyAdapter struct{}

func (skKeyAdapter) Type() string { return store.UpstreamSKKey }

func (skKeyAdapter) keyHeader(s Site) map[string]string {
	return map[string]string{"Authorization": "Bearer " + s.Cred.Key}
}

func (skKeyAdapter) Probe(ctx context.Context, s Site) (ProbeResult, error) {
	return probeNewAPILike(ctx, s, store.UpstreamSKKey, "sk-key")
}

func (a skKeyAdapter) Verify(ctx context.Context, s Site) error {
	var sub subscription
	return getJSON(ctx, s.client(), joinURL(s.BaseURL, "/v1/dashboard/billing/subscription"), a.keyHeader(s), &sub)
}

type subscription struct {
	HardLimitUSD float64 `json:"hard_limit_usd"`
	Error        *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Balance follows NewAPI controller/billing.go:
//   - subscription.hard_limit_usd = (remain_quota + used_quota) / quota_per_unit (total granted)
//     for the token (or the user when DisplayTokenStatEnabled is off); 100000000 for unlimited tokens.
//   - usage.total_usage = used_quota / quota_per_unit * 100 (i.e. USD cents).
//
// remaining USD = hard_limit_usd - total_usage/100.
func (a skKeyAdapter) Balance(ctx context.Context, s Site) (Balance, error) {
	hdr := a.keyHeader(s)
	var sub subscription
	if err := getJSON(ctx, s.client(), joinURL(s.BaseURL, "/v1/dashboard/billing/subscription"), hdr, &sub); err != nil {
		return Balance{}, fmt.Errorf("billing subscription: %w", err)
	}
	if sub.Error != nil {
		return Balance{}, &apiError{msg: "billing subscription: " + sub.Error.Message}
	}
	if sub.HardLimitUSD >= unlimitedThresholdUSD {
		return Balance{Unlimited: true, Raw: fmt.Sprintf("hard_limit_usd=%g", sub.HardLimitUSD)}, nil
	}
	var usage struct {
		TotalUsage float64 `json:"total_usage"`
		Error      *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	usageURL := joinURL(s.BaseURL, "/v1/dashboard/billing/usage")
	if err := getJSON(ctx, s.client(), usageURL, hdr, &usage); err != nil {
		return Balance{}, fmt.Errorf("billing usage: %w", err)
	}
	if usage.Error != nil {
		return Balance{}, &apiError{msg: "billing usage: " + usage.Error.Message}
	}
	return Balance{
		USD: sub.HardLimitUSD - usage.TotalUsage/100,
		Raw: fmt.Sprintf("hard_limit_usd=%g total_usage=%g", sub.HardLimitUSD, usage.TotalUsage),
	}, nil
}

func (skKeyAdapter) ListTokens(_ context.Context, s Site) ([]Token, error) {
	t, err := singleKeyToken(s)
	if err != nil {
		return nil, err
	}
	return []Token{t}, nil
}

func (skKeyAdapter) CreateToken(context.Context, Site, TokenCreate) (Token, error) {
	return Token{}, fmt.Errorf("sk-key create token: %w", ErrUnsupported)
}

func (skKeyAdapter) Pricing(context.Context, Site) (Pricing, error) {
	return Pricing{}, fmt.Errorf("sk-key pricing: %w", ErrUnsupported)
}

func (skKeyAdapter) Announcements(ctx context.Context, s Site) (Announcement, error) {
	var text string
	c := &naSession{hc: s.client(), base: s.BaseURL, hdr: map[string]string{}}
	if err := c.get(ctx, "/api/notice", &text); err != nil {
		return Announcement{}, fmt.Errorf("sk-key notice (%v): %w", err, ErrUnsupported)
	}
	return Announcement{Content: text}, nil
}

func (skKeyAdapter) ListModels(ctx context.Context, s Site, key string) ([]string, error) {
	return listModelsOpenAI(ctx, s, key)
}
