package core_test

import (
	"math"
	"strconv"
	"testing"
)

func TestM2GenericBalance(t *testing.T) {
	m := newM2(t)
	cfg := `{"balance":{"url":"/_custom/bal","json_path":"data.left","factor":0.01}}`
	if r := m.call("POST", "/upstreams", map[string]any{"name": "g", "type": "generic", "base_url": m.site.URL, "adapter_config": `{"balance":{"url":""}}`}); r.Code != 400 {
		t.Fatalf("invalid generic config: %d %s", r.Code, r.Raw)
	}
	var up struct {
		ID uint `json:"id"`
	}
	m.must("POST", "/upstreams", map[string]any{"name": "g", "type": "generic", "base_url": m.site.URL, "adapter_config": cfg}).into(t, &up)
	base := "/upstreams/" + strconv.Itoa(int(up.ID))
	m.must("POST", base+"/sync", map[string]any{"what": []string{"balance"}})
	var got struct {
		BalanceUSD float64 `json:"balance_usd"`
	}
	m.must("GET", base, nil).into(t, &got)
	if math.Abs(got.BalanceUSD-123.45) > 1e-6 {
		t.Fatalf("balance: %v", got.BalanceUSD)
	}
	// unsupported parts are a no-op, not an error
	var res []struct {
		OK bool `json:"ok"`
	}
	m.must("POST", base+"/sync", map[string]any{"what": []string{"groups"}}).into(t, &res)
	if len(res) != 1 || !res[0].OK {
		t.Fatalf("groups on generic: %+v", res)
	}
	// wizard checks without saving
	var checks []struct {
		Name    string `json:"name"`
		OK      bool   `json:"ok"`
		Skipped bool   `json:"skipped"`
	}
	m.must("POST", "/upstreams/test", map[string]any{"type": "generic", "base_url": m.site.URL, "adapter_config": cfg}).into(t, &checks)
	if len(checks) != 5 || !checks[0].OK || !checks[1].OK || !checks[3].Skipped {
		t.Fatalf("checks: %+v", checks)
	}
	var pr struct {
		Type    string `json:"type"`
		Adapter string `json:"adapter"`
	}
	m.must("POST", "/upstreams/probe", map[string]any{"base_url": m.site.URL}).into(t, &pr)
	if pr.Type != "newapi" || pr.Adapter != "newapi" {
		t.Fatalf("probe: %+v", pr)
	}
}
