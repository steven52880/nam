package core_test

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/steven52880/nam/core"
	"github.com/steven52880/nam/ports/portstest"
)

const litellmDoc = `{"test-model":{"input_cost_per_token":3e-6,"output_cost_per_token":1.5e-5,` +
	`"cache_read_input_token_cost":3e-7,"cache_creation_input_token_cost":3.75e-6,"mode":"chat","litellm_provider":"anthropic"}}`

// newM3 is newM2 with the price-book URL pointed at an httptest LiteLLM server.
func newM3(t *testing.T) *m2 {
	t.Helper()
	lite := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(litellmDoc)) }))
	t.Cleanup(lite.Close)
	fake := portstest.NewFake()
	c, err := core.New(core.Config{DBPath: filepath.Join(t.TempDir(), "nam.db"), Secret: "test-secret", PriceBookURL: lite.URL}, fake.Ports())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Stop)
	runCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	c.Start(runCtx)
	site := httptest.NewServer(newFakeSite().handler())
	t.Cleanup(site.Close)
	return &m2{t: t, h: c.Handler(), fake: fake, site: site}
}

// setupChannels creates upstream (currency 0.5) → sync → vendor → 3 applied channels (tokens a,b,c).
// Returns the upstream id and channel ids by token letter. No price book is loaded yet.
func (m *m2) setupChannels() (uint, map[string]int) {
	m.t.Helper()
	var up struct {
		ID uint `json:"id"`
	}
	m.must("POST", "/upstreams", map[string]any{"name": "mock-a", "type": "newapi", "base_url": m.site.URL, "cred_type": "access_token",
		"credential": map[string]any{"access_token": "tok"}, "platform_user_id": 42, "currency_rate": 0.5}).into(m.t, &up)
	base := "/upstreams/" + strconv.Itoa(int(up.ID))
	m.must("POST", base+"/sync", map[string]any{})
	var tokens []struct {
		ID uint `json:"id"`
	}
	m.must("GET", base+"/tokens", nil).into(m.t, &tokens)
	m.must("POST", "/vendors", map[string]any{"name": "anthropic", "group_name": "anthropic", "models": []string{"test-model", "mystery"}})
	ids := make([]uint, 0, len(tokens))
	for _, tk := range tokens {
		ids = append(ids, tk.ID)
	}
	tokenModels := map[string][]string{}
	for _, id := range ids {
		tokenModels[strconv.Itoa(int(id))] = []string{"test-model"}
	}
	req := map[string]any{"upstream_id": up.ID, "token_ids": ids, "token_models": tokenModels, "weight": 1}
	m.must("POST", "/channels/batch", req)
	m.must("POST", "/channels/apply", map[string]any{})
	live, _ := m.fake.List(context.Background())
	out := map[string]int{}
	for _, c := range live {
		for _, l := range []string{"a", "b", "c"} {
			if strings.HasSuffix(c.Name, "-"+l+"-anthropic") {
				out[l] = c.ID
			}
		}
	}
	if len(out) != 3 {
		m.t.Fatalf("channels: %+v", out)
	}
	return up.ID, out
}

type m3Row struct {
	ID             int      `json:"id"`
	Discount       float64  `json:"discount"`
	DiscountSource string   `json:"discount_source"`
	BalanceUSD     *float64 `json:"balance_usd"`
	BalanceCNY     *float64 `json:"balance_cny"`
	UnitCost       *float64 `json:"unit_cost"`
	UnitCostModel  string   `json:"unit_cost_model"`
}

func (m *m2) row(id int) m3Row {
	m.t.Helper()
	var d struct {
		Items []m3Row `json:"items"`
	}
	m.must("GET", "/channels", nil).into(m.t, &d)
	for _, r := range d.Items {
		if r.ID == id {
			return r
		}
	}
	m.t.Fatalf("channel %d not in /channels", id)
	return m3Row{}
}

func near(a, b, rel float64) bool { return math.Abs(a-b) <= rel*math.Abs(b) }

func (m *m2) jobMessage(name string) string {
	m.t.Helper()
	var js []struct {
		Name    string `json:"name"`
		LastMsg string `json:"last_message"`
	}
	m.must("GET", "/jobs", nil).into(m.t, &js)
	for _, j := range js {
		if j.Name == name {
			return j.LastMsg
		}
	}
	return ""
}
