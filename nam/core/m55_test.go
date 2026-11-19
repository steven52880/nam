package core_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/steven52880/nam/core"
	"github.com/steven52880/nam/ports"
	"github.com/steven52880/nam/ports/portstest"
)

// newM55 is newM3 whose upstream /api/pricing reports a model ratio the test can change.
func newM55(t *testing.T) (*m2, *atomic.Int64) {
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

	var ratioX10 atomic.Int64 // model_ratio × 10
	ratioX10.Store(15)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/pricing", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true, "group_ratio": map[string]any{"a": 0.4, "b": 0.5},
			"data": []map[string]any{{"model_name": "test-model", "quota_type": 0, "model_ratio": float64(ratioX10.Load()) / 10, "completion_ratio": 5,
				"enable_groups": []string{"a", "b"}}},
		})
	})
	mux.Handle("/", newFakeSite().handler())
	site := httptest.NewServer(mux)
	t.Cleanup(site.Close)
	return &m2{t: t, h: c.Handler(), fake: fake, site: site}, &ratioX10
}

// 选完上游和令牌立刻有倍率；改上游价格后立刻变化（同步完就变，不需要任何推算任务）。
func TestM55ChannelDiscountIsComputedOnRead(t *testing.T) {
	m, ratioX10 := newM55(t)
	up, ch := m.setupChannels()
	a, c := ch["a"], ch["c"]

	// no price book yet → model price cannot be divided → falls back to the upstream group ratio
	if r := m.row(a); r.DiscountSource != "group" || r.Discount == nil || !near(*r.Discount, 0.4, 1e-9) || r.GroupRatio == nil || *r.GroupRatio != 0.4 {
		t.Fatalf("group fallback: %+v", r)
	}
	// token group c does not exist upstream: no discount, no cost, with a reason
	if r := m.row(c); r.Discount != nil || r.DiscountSource != "none" || r.DiscountReason != "上游没有这个分组的价格和倍率" ||
		r.ChannelCost != nil || r.UnitCost != nil || r.GroupRatio != nil {
		t.Fatalf("none: %+v", r)
	}

	// price book arrives: the discount is now the median of 上游价 ÷ 原厂价 = 1.5 × 2 × 0.4 ÷ 3
	m.must("POST", "/jobs/price_book?wait=1", nil)
	r := m.row(a)
	if r.DiscountSource != "auto" || r.Discount == nil || !near(*r.Discount, 0.4, 1e-6) || r.UnitCost == nil || !near(*r.UnitCost, 0.00996*0.4*0.5, 1e-6) {
		t.Fatalf("auto: %+v", r)
	}
	base := "/channels/" + strconv.Itoa(a)
	var md struct {
		Discount      *float64 `json:"discount"`
		Source        string   `json:"discount_source"`
		GroupRatio    *float64 `json:"upstream_group_ratio"`
		DerivedMedian *float64 `json:"derived_median"`
		Models        []struct {
			Effective *float64 `json:"effective"`
			Source    string   `json:"effective_source"`
			Deviates  bool     `json:"deviates"`
		} `json:"models"`
	}
	m.must("GET", base+"/models", nil).into(t, &md)
	if md.Source != "auto" || md.GroupRatio == nil || md.DerivedMedian == nil || len(md.Models) != 1 || md.Models[0].Source != "model" || md.Models[0].Deviates {
		t.Fatalf("models before: %+v", md)
	}

	// the upstream raises the model price (ratio 1.5 → 3.0); one upstream sync later the row shows it
	ratioX10.Store(30)
	m.must("POST", "/upstreams/"+strconv.Itoa(int(up))+"/sync", map[string]any{})
	r = m.row(a)
	if r.DiscountSource != "auto" || r.Discount == nil || !near(*r.Discount, 0.8, 1e-6) || r.UnitCost == nil || !near(*r.UnitCost, 0.00996*0.8*0.5, 1e-6) {
		t.Fatalf("after price change: %+v", r)
	}
	// the model is 100% above the group ratio 0.4: flagged
	m.must("GET", base+"/models", nil).into(t, &md)
	if md.Models[0].Effective == nil || !near(*md.Models[0].Effective, 0.8, 1e-6) || !md.Models[0].Deviates {
		t.Fatalf("models after: %+v", md)
	}
	// manual wins, "auto" clears it again
	m.must("PATCH", "/channels/drafts", map[string]any{"changes": []map[string]any{{"channel_id": a, "ext": map[string]any{"discount": 0.35}}}})
	if r := m.row(a); r.DiscountSource != "manual" || r.Discount == nil || *r.Discount != 0.35 {
		t.Fatalf("manual draft: %+v", r)
	}
	m.must("POST", "/channels/apply", map[string]any{})
	m.must("PATCH", "/channels/drafts", map[string]any{"changes": []map[string]any{{"channel_id": a, "ext": map[string]any{"discount_source": "auto"}}}})
	m.must("POST", "/channels/apply", map[string]any{})
	if r := m.row(a); r.DiscountSource != "auto" || !near(*r.Discount, 0.8, 1e-6) {
		t.Fatalf("manual cleared: %+v", r)
	}
}

func TestM55ChannelIssues(t *testing.T) {
	m, _ := newM55(t)
	_, ch := m.setupChannels()
	m.must("POST", "/vendors", map[string]any{"name": "gemini", "group_name": "gemini", "models": []string{"gem-x"}, "channel_type": 24})
	type issues struct {
		Items []struct {
			ChannelID int    `json:"channel_id"`
			Name      string `json:"name"`
			Field     string `json:"field"`
			Message   string `json:"message"`
		} `json:"items"`
	}
	var is issues
	m.must("GET", "/channels/issues", nil).into(t, &is)
	if is.Items == nil || len(is.Items) != 0 {
		t.Fatalf("clean: %+v", is)
	}
	// someone puts channel a into a second vendor group directly in NewAPI: only the rule check can see it
	g := "anthropic,gemini"
	if err := m.fake.Update(context.Background(), ch["a"], ports.ChannelPatch{Group: &g}); err != nil {
		t.Fatal(err)
	}
	m.must("GET", "/channels/issues", nil).into(t, &is)
	// the channel itself is reported for the second vendor group; its model now sits in two groups, which the
	// other channels carrying it are told about as well
	var groupIssue bool
	for _, it := range is.Items {
		if it.Name == "" || it.Message == "" {
			t.Fatalf("incomplete item: %+v", it)
		}
		if it.ChannelID == ch["a"] && it.Field == "group" {
			groupIssue = true
		}
	}
	if !groupIssue || len(is.Items) < 2 {
		t.Fatalf("issues: %+v", is)
	}
	n := len(is.Items)
	m.must("POST", "/jobs/consistency?wait=1", nil)
	if msg := m.jobMessage("consistency"); msg != "检查 3 个渠道，规则问题 "+strconv.Itoa(n)+" 条，自动禁用 0 个" {
		t.Fatalf("summary: %q", msg)
	}
}
