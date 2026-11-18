package pricing

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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

// officialPrefixes lists the LiteLLM key prefixes that belong to the model's
// original vendor, in priority order. Resale/gateway/regional providers
// (aihubmix, openrouter, bedrock, azure, ...) are deliberately absent.
var officialPrefixes = []string{
	"anthropic", "openai", "gemini", "deepseek", "xai",
	"moonshot", "zai", "dashscope", "minimax", "mistral",
}

// lookupOfficial finds the original vendor's price for model: a key identical
// to the model name, else "<official prefix>/<model>". Matching is exact and
// case-sensitive; anything else is reported as not found.
func lookupOfficial(prices map[string]store.OfficialPrice, model string) (store.OfficialPrice, bool) {
	if p, ok := prices[model]; ok {
		return p, true
	}
	for _, prefix := range officialPrefixes {
		if p, ok := prices[prefix+"/"+model]; ok {
			return p, true
		}
	}
	return store.OfficialPrice{}, false
}

// ParseLiteLLM parses a model_prices_and_context_window.json document into a
// map keyed by the full LiteLLM key. Entries without a usable price are skipped.
func ParseLiteLLM(r io.Reader) (map[string]store.OfficialPrice, error) {
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse litellm json: %w", err)
	}
	out := make(map[string]store.OfficialPrice, len(raw))
	for key, body := range raw {
		if key == "" || key == "sample_spec" {
			continue
		}
		var e litellmEntry
		if err := json.Unmarshal(body, &e); err != nil {
			continue
		}
		if p, ok := e.price(); ok {
			out[key] = p
		}
	}
	return out, nil
}