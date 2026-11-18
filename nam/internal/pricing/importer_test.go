package pricing

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/steven52880/nam/internal/store"
)

func ptrNear(t *testing.T, label string, got, want *float64) {
	t.Helper()
	if got == nil && want == nil {
		return
	}
	if got == nil || want == nil {
		t.Fatalf("%s = %s, want %s", label, fmtPtr(got), fmtPtr(want))
	}
	if math.Abs(*got-*want) > 1e-9 {
		t.Fatalf("%s = %v, want %v", label, *got, *want)
	}
}

func fmtPtr(v *float64) string {
	if v == nil {
		return "nil"
	}
	return fmt.Sprint(*v)
}

func countModel(t *testing.T, st *store.Store, model string) int64 {
	t.Helper()
	var n int64
	if err := st.DB.Model(&store.OfficialPrice{}).Where("model = ?", model).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func samplePrices(t *testing.T) map[string]store.OfficialPrice {
	t.Helper()
	raw, err := os.ReadFile("testdata/litellm_sample.json")
	if err != nil {
		t.Fatal(err)
	}
	prices, err := ParseLiteLLM(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return prices
}

func TestParseLiteLLMMapping(t *testing.T) {
	prices := samplePrices(t)
	if len(prices) != 8 {
		t.Fatalf("parsed %d entries, want 8", len(prices))
	}

	claude := prices["claude-sonnet-4-5"]
	ptrNear(t, "claude input", claude.Input, f(3))
	ptrNear(t, "claude output", claude.Output, f(15))
	ptrNear(t, "claude cache read", claude.CacheRead, f(0.3))
	ptrNear(t, "claude cache write 5m", claude.CacheWrite5m, f(3.75))
	ptrNear(t, "claude cache write 1h", claude.CacheWrite1h, f(6))
	if claude.Vendor != "anthropic" || claude.Source != "litellm" {
		t.Fatalf("claude vendor/source = %q/%q", claude.Vendor, claude.Source)
	}

	// Keys stay as published: no provider-stripped aliases are invented.
	ptrNear(t, "gpt-4o input", prices["gpt-4o"].Input, f(2.5))
	ptrNear(t, "azure/gpt-4o input", prices["azure/gpt-4o"].Input, f(5))
	ptrNear(t, "gemini/ input", prices["gemini/gemini-2.5-pro"].Input, f(1.25))
	ptrNear(t, "vertex_ai exact input", prices["vertex_ai/gemini-2.5-pro"].Input, f(2.5))
	ptrNear(t, "openrouter/deepseek/deepseek-chat input", prices["openrouter/deepseek/deepseek-chat"].Input, f(1))
	for _, alias := range []string{"gemini-2.5-pro", "deepseek-chat"} {
		if _, exists := prices[alias]; exists {
			t.Fatalf("%s alias should not be registered", alias)
		}
	}

	img := prices["dall-e-3"]
	ptrNear(t, "dall-e-3 per request", img.PerRequest, f(0.04))
	if img.Input != nil {
		t.Fatalf("dall-e-3 input = %v, want nil", *img.Input)
	}

	for _, skipped := range []string{"sample_spec", "text-embedding-3-small", "free-model"} {
		if _, exists := prices[skipped]; exists {
			t.Fatalf("%s should be skipped", skipped)
		}
	}
}

func TestLookupOfficial(t *testing.T) {
	const doc = `{
	  "claude-x": {"input_cost_per_token": 1e-06, "mode": "chat", "litellm_provider": "anthropic"},
	  "anthropic/claude-x": {"input_cost_per_token": 7e-06, "mode": "chat", "litellm_provider": "anthropic"},
	  "openai/claude-x": {"input_cost_per_token": 9e-06, "mode": "chat", "litellm_provider": "openai"},
	  "openai/m-order": {"input_cost_per_token": 2e-06, "mode": "chat", "litellm_provider": "openai"},
	  "anthropic/m-order": {"input_cost_per_token": 3e-06, "mode": "chat", "litellm_provider": "anthropic"},
	  "moonshot/kimi-a": {"input_cost_per_token": 4e-06, "mode": "chat", "litellm_provider": "moonshot"},
	  "dashscope/qwen-a": {"input_cost_per_token": 5e-06, "mode": "chat", "litellm_provider": "dashscope"},
	  "Mixed-Case": {"input_cost_per_token": 6e-06, "mode": "chat", "litellm_provider": "openai"},
	  "aihubmix/kimi-b": {"input_cost_per_token": 1e-06, "mode": "chat", "litellm_provider": "aihubmix"},
	  "fal_ai/img-b": {"output_cost_per_image": 0.1, "mode": "image_generation", "litellm_provider": "fal_ai"},
	  "github_copilot/m-b": {"input_cost_per_token": 1e-06, "mode": "chat", "litellm_provider": "github_copilot"},
	  "azure_ai/m-c": {"input_cost_per_token": 1e-06, "mode": "chat", "litellm_provider": "azure_ai"},
	  "openrouter/m-d": {"input_cost_per_token": 1e-06, "mode": "chat", "litellm_provider": "openrouter"},
	  "bedrock/m-e": {"input_cost_per_token": 1e-06, "mode": "chat", "litellm_provider": "bedrock"},
	  "vertex_ai/m-f": {"input_cost_per_token": 1e-06, "mode": "chat", "litellm_provider": "vertex_ai"}
	}`
	prices, err := ParseLiteLLM(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		model string
		want  float64 // USD/1M input; 0 means "must not match"
	}{
		{"claude-x", 1},       // identical key beats official prefixes
		{"m-order", 3},        // anthropic precedes openai in the whitelist
		{"kimi-a", 4},         // moonshot prefix
		{"qwen-a", 5},         // dashscope prefix
		{"Mixed-Case", 6},     // exact, case-sensitive
		{"mixed-case", 0},     // different case does not match
		{"kimi-b", 0},         // aihubmix is a reseller
		{"img-b", 0},          // fal_ai is a reseller
		{"m-b", 0},            // github_copilot
		{"m-c", 0},            // azure_ai
		{"m-d", 0},            // openrouter
		{"m-e", 0},            // bedrock
		{"m-f", 0},            // vertex_ai
		{"openai/m-order", 2}, // full key is still an exact match
	} {
		p, ok := lookupOfficial(prices, tc.model)
		if tc.want == 0 {
			if ok {
				t.Errorf("%s matched (vendor %q), want no match", tc.model, p.Vendor)
			}
			continue
		}
		if !ok {
			t.Errorf("%s: no match, want input %v", tc.model, tc.want)
			continue
		}
		ptrNear(t, tc.model+" input", p.Input, f(tc.want))
	}
}
func TestImportFilePlanFixture(t *testing.T) {
	st := openStore(t)
	const doc = `{
	  "test-model": {
	    "input_cost_per_token": 3e-06,
	    "output_cost_per_token": 1.5e-05,
	    "cache_read_input_token_cost": 3e-07,
	    "cache_creation_input_token_cost": 3.75e-06,
	    "litellm_provider": "openai",
	    "mode": "chat"
	  }
	}`
	im := &Importer{St: st}
	res, err := im.ImportFile(context.Background(), strings.NewReader(doc), []string{"test-model"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Fetched != 1 || res.Updated != 1 || len(res.Missing) != 0 {
		t.Fatalf("result = %+v", res)
	}
	var got store.OfficialPrice
	if err := st.DB.First(&got, "model = ?", "test-model").Error; err != nil {
		t.Fatal(err)
	}
	ptrNear(t, "test-model input", got.Input, f(3))
	ptrNear(t, "test-model output", got.Output, f(15))
	ptrNear(t, "test-model cache read", got.CacheRead, f(0.3))
	ptrNear(t, "test-model cache write 5m", got.CacheWrite5m, f(3.75))
	if got.CacheWrite1h != nil {
		t.Fatalf("test-model cache write 1h = %v, want nil", *got.CacheWrite1h)
	}
	if got.Source != "litellm" || got.Vendor != "openai" {
		t.Fatalf("test-model source/vendor = %q/%q", got.Source, got.Vendor)
	}
}

func TestSyncFromLiteLLMServer(t *testing.T) {
	st := openStore(t)
	raw, err := os.ReadFile("testdata/litellm_sample.json")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(raw)
	}))
	defer srv.Close()

	im := &Importer{St: st, HTTP: srv.Client(), LiteLLMURL: srv.URL}
	models := []string{"claude-sonnet-4-5", "gemini-2.5-pro", "deepseek-chat", "GPT-4O", "zzz-missing", "aaa-missing"}
	res, err := im.Sync(context.Background(), models)
	if err != nil {
		t.Fatal(err)
	}
	if res.Fetched != 8 || res.Updated != 3 {
		t.Fatalf("result = %+v", res)
	}
	want := []string{"GPT-4O", "aaa-missing", "zzz-missing"}
	if len(res.Missing) != len(want) {
		t.Fatalf("missing = %v, want %v", res.Missing, want)
	}
	for i := range want {
		if res.Missing[i] != want[i] {
			t.Fatalf("missing = %v, want %v", res.Missing, want)
		}
	}

	var count int64
	if err := st.DB.Model(&store.OfficialPrice{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("stored %d rows, want 3", count)
	}
	var gm store.OfficialPrice
	if err := st.DB.First(&gm, "model = ?", "gemini-2.5-pro").Error; err != nil {
		t.Fatal(err)
	}
	ptrNear(t, "synced gemini input", gm.Input, f(1.25))
	if gm.Vendor != "gemini" || gm.Source != "litellm" {
		t.Fatalf("gemini vendor/source = %q/%q", gm.Vendor, gm.Source)
	}
	if n := countModel(t, st, "GPT-4O"); n != 0 {
		t.Fatalf("case-different model stored %d rows, want 0", n)
	}
}

func TestSyncDropsStaleMismatchedRows(t *testing.T) {
	st := openStore(t)
	// Rows left behind by the old fuzzy/reseller matching, plus a manual row.
	for _, row := range []store.OfficialPrice{
		{Model: "kimi-k3", Vendor: "aihubmix", Source: "litellm", Input: f(1)},
		{Model: "gpt-4o", Vendor: "azure", Source: "litellm", Input: f(5)},
		{Model: "manual-keep", Source: "manual", Input: f(7)},
	} {
		row := row
		if err := st.DB.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	const doc = `{
	  "gpt-4o": {"input_cost_per_token": 2.5e-06, "mode": "chat", "litellm_provider": "openai"},
	  "aihubmix/kimi-k3": {"input_cost_per_token": 1e-06, "mode": "chat", "litellm_provider": "aihubmix"}
	}`
	im := &Importer{St: st}
	res, err := im.ImportFile(context.Background(), strings.NewReader(doc),
		[]string{"kimi-k3", "gpt-4o", "manual-keep"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Missing) != 2 || res.Missing[0] != "kimi-k3" || res.Missing[1] != "manual-keep" {
		t.Fatalf("missing = %v", res.Missing)
	}
	if n := countModel(t, st, "kimi-k3"); n != 0 {
		t.Fatalf("stale kimi-k3 rows = %d, want 0", n)
	}
	if n := countModel(t, st, "manual-keep"); n != 1 {
		t.Fatalf("manual row count = %d, want 1", n)
	}
	var gpt store.OfficialPrice
	if err := st.DB.First(&gpt, "model = ?", "gpt-4o").Error; err != nil {
		t.Fatal(err)
	}
	ptrNear(t, "gpt-4o corrected input", gpt.Input, f(2.5))
	if gpt.Vendor != "openai" {
		t.Fatalf("gpt-4o vendor = %q", gpt.Vendor)
	}
}

func TestSyncKeepsManualAndDeletesStale(t *testing.T) {
	st := openStore(t)
	if err := st.DB.Create(&store.OfficialPrice{Model: "manual-keep", Source: "manual", Input: f(7)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := st.DB.Create(&store.OfficialPrice{Model: "stale-auto", Source: "litellm", Input: f(1)}).Error; err != nil {
		t.Fatal(err)
	}
	const doc = `{"gpt-4o": {"input_cost_per_token": 2.5e-06, "output_cost_per_token": 1e-05, "mode": "chat", "litellm_provider": "openai"}}`
	im := &Importer{St: st}
	res, err := im.ImportFile(context.Background(), strings.NewReader(doc), []string{"gpt-4o"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Updated != 1 {
		t.Fatalf("updated = %d, want 1", res.Updated)
	}
	if n := countModel(t, st, "manual-keep"); n != 1 {
		t.Fatalf("manual row count = %d, want 1", n)
	}
	if n := countModel(t, st, "stale-auto"); n != 0 {
		t.Fatalf("stale auto row count = %d, want 0", n)
	}
	var manual store.OfficialPrice
	if err := st.DB.First(&manual, "model = ?", "manual-keep").Error; err != nil {
		t.Fatal(err)
	}
	ptrNear(t, "manual input kept", manual.Input, f(7))
}

func TestSyncUpdatesChangedPrice(t *testing.T) {
	st := openStore(t)
	var body atomic.Value
	body.Store(`{"gpt-4o": {"input_cost_per_token": 2.5e-06, "output_cost_per_token": 1e-05, "mode": "chat", "litellm_provider": "openai"}}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body.Load().(string)))
	}))
	defer srv.Close()

	im := &Importer{St: st, HTTP: srv.Client(), LiteLLMURL: srv.URL}
	ctx := context.Background()
	if res, err := im.Sync(ctx, []string{"gpt-4o"}); err != nil || res.Updated != 1 {
		t.Fatalf("first sync = %+v, %v", res, err)
	}
	body.Store(`{"gpt-4o": {"input_cost_per_token": 3e-06, "output_cost_per_token": 1.2e-05, "mode": "chat", "litellm_provider": "openai"}}`)
	res, err := im.Sync(ctx, []string{"gpt-4o"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Updated != 1 {
		t.Fatalf("second sync updated = %d, want 1", res.Updated)
	}
	var got store.OfficialPrice
	if err := st.DB.First(&got, "model = ?", "gpt-4o").Error; err != nil {
		t.Fatal(err)
	}
	ptrNear(t, "changed input", got.Input, f(3))
	ptrNear(t, "changed output", got.Output, f(12))

	res, err = im.Sync(ctx, []string{"gpt-4o"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Updated != 0 {
		t.Fatalf("third sync updated = %d, want 0", res.Updated)
	}
}
