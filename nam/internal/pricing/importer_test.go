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
	if len(prices) != 10 {
		t.Fatalf("parsed %d entries, want 10", len(prices))
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

	// Unprefixed original wins over a regional copy.
	ptrNear(t, "gpt-4o input", prices["gpt-4o"].Input, f(2.5))
	ptrNear(t, "azure/gpt-4o input", prices["azure/gpt-4o"].Input, f(5))

	// Provider-prefixed-only entries register the unprefixed alias, and gemini
	// wins over vertex_ai for the same model.
	gemini, ok := prices["gemini-2.5-pro"]
	if !ok {
		t.Fatal("gemini-2.5-pro alias missing")
	}
	ptrNear(t, "gemini alias input", gemini.Input, f(1.25))
	if gemini.Vendor != "gemini" {
		t.Fatalf("gemini alias vendor = %q", gemini.Vendor)
	}
	ptrNear(t, "vertex_ai exact input", prices["vertex_ai/gemini-2.5-pro"].Input, f(2.5))

	deepseek, ok := prices["deepseek-chat"]
	if !ok {
		t.Fatal("deepseek-chat alias missing")
	}
	ptrNear(t, "deepseek alias input", deepseek.Input, f(0.28))
	if deepseek.Vendor != "deepseek" {
		t.Fatalf("deepseek alias vendor = %q", deepseek.Vendor)
	}
	ptrNear(t, "openrouter/deepseek/deepseek-chat input", prices["openrouter/deepseek/deepseek-chat"].Input, f(1))

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

func TestParseLiteLLMProviderPreference(t *testing.T) {
	const doc = `{
	  "openai/claude-x": {"input_cost_per_token": 9e-06, "mode": "chat", "litellm_provider": "openai"},
	  "anthropic/claude-x": {"input_cost_per_token": 3e-06, "mode": "chat", "litellm_provider": "anthropic"},
	  "bedrock/claude-y": {"input_cost_per_token": 9e-06, "mode": "chat", "litellm_provider": "bedrock"},
	  "openai/claude-y": {"input_cost_per_token": 2e-06, "mode": "chat", "litellm_provider": "openai"},
	  "azure/claude-z": {"input_cost_per_token": 4e-06, "mode": "chat", "litellm_provider": "azure"},
	  "claude-w": {"input_cost_per_token": 1e-06, "mode": "chat", "litellm_provider": "anthropic"},
	  "anthropic/claude-w": {"input_cost_per_token": 7e-06, "mode": "chat", "litellm_provider": "anthropic"}
	}`
	prices, err := ParseLiteLLM(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	ptrNear(t, "anthropic beats openai", prices["claude-x"].Input, f(3))
	ptrNear(t, "openai beats bedrock", prices["claude-y"].Input, f(2))
	ptrNear(t, "azure fallback", prices["claude-z"].Input, f(4))
	ptrNear(t, "unprefixed wins", prices["claude-w"].Input, f(1))
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
	if res.Fetched != 10 || res.Updated != 4 {
		t.Fatalf("result = %+v", res)
	}
	if len(res.Missing) != 2 || res.Missing[0] != "aaa-missing" || res.Missing[1] != "zzz-missing" {
		t.Fatalf("missing = %v", res.Missing)
	}

	var count int64
	if err := st.DB.Model(&store.OfficialPrice{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Fatalf("stored %d rows, want 4", count)
	}
	var gm store.OfficialPrice
	if err := st.DB.First(&gm, "model = ?", "gemini-2.5-pro").Error; err != nil {
		t.Fatal(err)
	}
	ptrNear(t, "synced gemini input", gm.Input, f(1.25))
	if gm.Vendor != "gemini" || gm.Source != "litellm" {
		t.Fatalf("gemini vendor/source = %q/%q", gm.Vendor, gm.Source)
	}
	var upper store.OfficialPrice
	if err := st.DB.First(&upper, "model = ?", "GPT-4O").Error; err != nil {
		t.Fatal(err)
	}
	ptrNear(t, "lowercase-matched input", upper.Input, f(2.5))
	if upper.Vendor != "openai" {
		t.Fatalf("GPT-4O vendor = %q", upper.Vendor)
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
