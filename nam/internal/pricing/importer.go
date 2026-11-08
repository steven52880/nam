package pricing

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/steven52880/nam/internal/store"
)

// DefaultLiteLLMURL is BerriAI's published price list.
const DefaultLiteLLMURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"

const litellmSource = "litellm"

// Importer downloads LiteLLM's price list and keeps official_price in sync with
// the models the site actually serves.
type Importer struct {
	St         *store.Store
	HTTP       *http.Client
	LiteLLMURL string
}

// SyncResult summarizes one import.
type SyncResult struct {
	Fetched int      `json:"fetched"`
	Updated int      `json:"updated"`
	Missing []string `json:"missing"`
}

type litellmEntry struct {
	InputCostPerToken                   *float64 `json:"input_cost_per_token"`
	OutputCostPerToken                  *float64 `json:"output_cost_per_token"`
	CacheReadInputTokenCost             *float64 `json:"cache_read_input_token_cost"`
	CacheCreationInputTokenCost         *float64 `json:"cache_creation_input_token_cost"`
	CacheCreationInputTokenCostAbove1hr *float64 `json:"cache_creation_input_token_cost_above_1hr"`
	InputCostPerRequest                 *float64 `json:"input_cost_per_request"`
	InputCostPerQuery                   *float64 `json:"input_cost_per_query"`
	OutputCostPerImage                  *float64 `json:"output_cost_per_image"`
	LitellmProvider                     string   `json:"litellm_provider"`
	Mode                                string   `json:"mode"`
}

// price converts one LiteLLM entry to the book's units. Token prices are
// USD/token in LiteLLM and USD/1M in the book; per-request prices are kept as-is.
func (e litellmEntry) price() (store.OfficialPrice, bool) {
	p := store.OfficialPrice{Vendor: e.LitellmProvider, Source: litellmSource}
	switch e.Mode {
	case "chat", "completion", "responses", "":
		p.Input = perMillion(e.InputCostPerToken)
		p.Output = perMillion(e.OutputCostPerToken)
		p.CacheRead = perMillion(e.CacheReadInputTokenCost)
		p.CacheWrite5m = perMillion(e.CacheCreationInputTokenCost)
		p.CacheWrite1h = perMillion(e.CacheCreationInputTokenCostAbove1hr)
	case "image_generation", "search":
		p.PerRequest = firstPositive(e.InputCostPerRequest, e.OutputCostPerImage, e.InputCostPerQuery)
	default:
		return store.OfficialPrice{}, false
	}
	if !priced(p) {
		return store.OfficialPrice{}, false
	}
	return p, true
}

func perMillion(v *float64) *float64 {
	if v == nil {
		return nil
	}
	m := *v * 1e6
	return &m
}

func firstPositive(vs ...*float64) *float64 {
	for _, v := range vs {
		if v != nil && *v > 0 {
			return v
		}
	}
	return nil
}

func priced(p store.OfficialPrice) bool {
	for _, v := range []*float64{p.Input, p.Output, p.CacheRead, p.CacheWrite5m, p.CacheWrite1h, p.PerRequest} {
		if v != nil && *v > 0 {
			return true
		}
	}
	return false
}

type priceCandidate struct {
	name  string
	rank  int
	price store.OfficialPrice
}

// ParseLiteLLM parses a model_prices_and_context_window.json document. It keeps
// full LiteLLM keys and also registers provider-stripped aliases ("gemini/x" →
// "x"), preferring direct keys and canonical providers over regional ones.
func ParseLiteLLM(r io.Reader) (map[string]store.OfficialPrice, error) {
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse litellm json: %w", err)
	}
	exact := make(map[string]store.OfficialPrice, len(raw))
	aliases := make(map[string]priceCandidate)
	for key, body := range raw {
		if key == "" || key == "sample_spec" {
			continue
		}
		var e litellmEntry
		if err := json.Unmarshal(body, &e); err != nil {
			continue
		}
		p, ok := e.price()
		if !ok {
			continue
		}
		exact[key] = p
		prefix, rest := splitPrefix(key)
		if prefix == "" {
			continue
		}
		c := priceCandidate{name: key, rank: providerRank(prefix), price: p}
		if cur, found := aliases[rest]; !found || aliasBetter(c, cur) {
			aliases[rest] = c
		}
	}
	out := make(map[string]store.OfficialPrice, len(exact)+len(aliases))
	for k, v := range exact {
		out[k] = v
	}
	for name, c := range aliases {
		if _, exists := out[name]; !exists {
			out[name] = c.price
		}
	}
	return out, nil
}

func splitPrefix(key string) (string, string) {
	i := strings.IndexByte(key, '/')
	if i <= 0 || i == len(key)-1 {
		return "", key
	}
	return key[:i], key[i+1:]
}

func providerRank(prefix string) int {
	switch {
	case prefix == "anthropic":
		return 0
	case prefix == "openai":
		return 1
	case prefix == "gemini":
		return 2
	case strings.HasPrefix(prefix, "vertex_ai"):
		return 3
	case prefix == "deepseek":
		return 4
	case prefix == "xai":
		return 5
	case strings.HasPrefix(prefix, "bedrock"),
		strings.HasPrefix(prefix, "azure"),
		strings.HasPrefix(prefix, "openrouter"):
		return 100
	default:
		return 50
	}
}

func aliasBetter(a, b priceCandidate) bool {
	if a.rank != b.rank {
		return a.rank < b.rank
	}
	return a.name < b.name
}
