package core_test

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

type layoutResp struct {
	Layout  json.RawMessage `json:"layout"`
	SavedAt int64           `json:"saved_at"`
}

func (m *m2) auditAll() int {
	m.t.Helper()
	var d struct {
		Total int `json:"total"`
	}
	m.must("GET", "/audit?limit=1", nil).into(m.t, &d)
	return d.Total
}

func TestM45UILayout(t *testing.T) {
	m := newM2(t)

	var got layoutResp
	m.must("GET", "/ui/layout", nil).into(t, &got)
	if string(got.Layout) != "null" || got.SavedAt != 0 {
		t.Fatalf("empty read: %+v", got)
	}

	before := m.auditAll()
	var saved struct {
		SavedAt int64 `json:"saved_at"`
	}
	m.must("PUT", "/ui/layout", map[string]any{"layout": map[string]any{"v": 1, "slots": map[string]any{"a": []int{1, 2}}}}).into(t, &saved)
	if saved.SavedAt <= 0 {
		t.Fatalf("saved_at: %+v", saved)
	}
	m.must("GET", "/ui/layout", nil).into(t, &got)
	if got.SavedAt != saved.SavedAt || string(got.Layout) != `{"slots":{"a":[1,2]},"v":1}` {
		t.Fatalf("read back: %s saved_at=%d", got.Layout, got.SavedAt)
	}
	// overwrite replaces the whole document
	m.must("PUT", "/ui/layout", map[string]any{"layout": map[string]any{"v": 2}})
	m.must("GET", "/ui/layout", nil).into(t, &got)
	if string(got.Layout) != `{"v":2}` {
		t.Fatalf("overwrite: %s", got.Layout)
	}
	if after := m.auditAll(); after != before {
		t.Fatalf("layout save must not write audit: %d → %d", before, after)
	}

	// a rejected save leaves the stored layout alone
	for name, body := range map[string]any{
		"array":   map[string]any{"layout": []int{1}},
		"string":  map[string]any{"layout": "x"},
		"number":  map[string]any{"layout": 1},
		"null":    map[string]any{"layout": nil},
		"missing": map[string]any{},
	} {
		if r := m.call("PUT", "/ui/layout", body); r.Code != 400 {
			t.Errorf("%s: want 400, got %d %s", name, r.Code, r.Raw)
		}
	}
	if w := do(m.h, "PUT", "/api/nam/ui/layout", "not json", "100"); w.Code != 400 {
		t.Errorf("garbage body: %d", w.Code)
	}
	big := map[string]any{"layout": map[string]any{"pad": strings.Repeat("x", 300<<10)}}
	r := m.call("PUT", "/ui/layout", big)
	if r.Code != 400 || !strings.Contains(r.Body.Message, "布局太大") {
		t.Fatalf("oversize: %d %s", r.Code, r.Raw)
	}
	m.must("GET", "/ui/layout", nil).into(t, &got)
	if string(got.Layout) != `{"v":2}` {
		t.Fatalf("layout changed by rejected saves: %s", got.Layout)
	}

	// trailing content after the first JSON value is rejected too, and size counts it
	for name, body := range map[string]string{
		"trailing whitespace > 256 KB": `{"layout":{"v":3}}` + strings.Repeat(" ", 300<<10),
		"two documents":                `{"layout":{"v":3}}{"layout":{"v":4}}`,
		"trailing garbage":             `{"layout":{"v":3}} garbage`,
	} {
		w := do(m.h, "PUT", "/api/nam/ui/layout", body, "100")
		if w.Code != 400 {
			t.Errorf("%s: want 400, got %d", name, w.Code)
		}
		if name == "trailing whitespace > 256 KB" && !strings.Contains(w.Body.String(), "布局太大") {
			t.Errorf("%s: want 布局太大, got %s", name, w.Body)
		}
	}
	m.must("GET", "/ui/layout", nil).into(t, &got)
	if string(got.Layout) != `{"v":2}` {
		t.Fatalf("layout overwritten by trailing-content saves: %s", got.Layout)
	}

	// unauthenticated
	if w := do(m.h, "GET", "/api/nam/ui/layout", "", ""); w.Code != 401 {
		t.Fatalf("anon: %d", w.Code)
	}
}

func TestM45ModelParamValidation(t *testing.T) {
	m := newM2(t)
	long := strings.Repeat("a", 201)
	ok200 := strings.Repeat("a", 200)
	for name, p := range map[string]string{
		"stats too long":     "/stats/channels?model=" + long,
		"costs missing":      "/channels/costs",
		"costs empty":        "/channels/costs?model=",
		"costs blank":        "/channels/costs?model=%20%20",
		"costs too long":     "/channels/costs?model=" + long,
		"costs long unicode": "/channels/costs?model=" + url.QueryEscape(strings.Repeat("模", 201)),
	} {
		if r := m.call("GET", p, nil); r.Code != 400 {
			t.Errorf("%s: want 400, got %d %s", name, r.Code, r.Raw)
		}
	}
	for name, p := range map[string]string{
		"stats 200 chars":      "/stats/channels?model=" + ok200,
		"stats blank is empty": "/stats/channels?model=%20%20",
		"stats 200 unicode":    "/stats/channels?model=" + url.QueryEscape(strings.Repeat("模", 200)),
		"costs 200 chars":      "/channels/costs?model=" + ok200,
	} {
		if r := m.call("GET", p, nil); r.Code != 200 {
			t.Errorf("%s: want 200, got %d %s", name, r.Code, r.Raw)
		}
	}
	var st struct {
		Model string `json:"model"`
	}
	m.must("GET", "/stats/channels?model=%20%20", nil).into(t, &st)
	if st.Model != "" {
		t.Fatalf("blank model echoed as %q", st.Model)
	}
	m.must("GET", "/stats/channels?model=%20x%20", nil).into(t, &st)
	if st.Model != "x" {
		t.Fatalf("model not trimmed: %q", st.Model)
	}
}

func TestM45StatsByModelAndCosts(t *testing.T) {
	m, a, b, _ := m4Fixture(t)
	const win = "?window=3d&model="
	var list struct {
		Model string `json:"model"`
		Items []struct {
			ChannelID int   `json:"channel_id"`
			Success   int64 `json:"success"`
		} `json:"items"`
	}
	m.must("GET", "/stats/channels"+win+"test-model", nil).into(t, &list)
	got := map[int]int64{}
	for _, it := range list.Items {
		got[it.ChannelID] = it.Success
	}
	if list.Model != "test-model" || len(got) != 2 || got[a] != 10 || got[b] != 5 {
		t.Fatalf("stats by model: %+v", list)
	}
	m.must("GET", "/stats/channels"+win+"mystery", nil).into(t, &list)
	if len(list.Items) != 0 {
		t.Fatalf("stats unknown model: %+v", list)
	}

	type cost struct {
		ChannelID   int      `json:"channel_id"`
		HasModel    bool     `json:"has_model"`
		Discount    *float64 `json:"discount"`
		ChannelCost *float64 `json:"channel_cost"`
		UnitCost    *float64 `json:"unit_cost"`
	}
	var costs struct {
		Model string `json:"model"`
		Items []cost `json:"items"`
	}
	m.must("GET", "/channels/costs?model=%20test-model%20", nil).into(t, &costs)
	if costs.Model != "test-model" || len(costs.Items) != 3 {
		t.Fatalf("costs: %+v", costs)
	}
	for _, c := range costs.Items {
		row := m.row(c.ChannelID)
		if !c.HasModel || c.Discount == nil || *c.Discount != row.Discount || c.UnitCost == nil || row.UnitCost == nil || *c.UnitCost != *row.UnitCost {
			t.Fatalf("channel %d: %+v vs row %+v", c.ChannelID, c, row)
		}
		if c.ChannelID == a || c.ChannelID == b {
			if !near(*c.ChannelCost, 0.175, 1e-9) { // discount 0.35 × upstream rate 0.5
				t.Fatalf("channel %d cost: %v", c.ChannelID, *c.ChannelCost)
			}
		}
	}
	m.must("GET", "/channels/costs?model=mystery", nil).into(t, &costs)
	if len(costs.Items) != 3 {
		t.Fatalf("costs mystery: %+v", costs)
	}
	for _, c := range costs.Items {
		if c.HasModel || c.Discount != nil || c.ChannelCost != nil || c.UnitCost != nil {
			t.Fatalf("channel without the model: %+v", c)
		}
	}
}
