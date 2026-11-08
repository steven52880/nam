package core_test

import (
	"strconv"
	"strings"
	"testing"
)

type officialDTO struct {
	Input        *float64 `json:"input"`
	Output       *float64 `json:"output"`
	CacheRead    *float64 `json:"cache_read"`
	CacheWrite5m *float64 `json:"cache_write_5m"`
	Source       string   `json:"source"`
}

type priceRowDTO struct {
	Model        string       `json:"model"`
	Official     *officialDTO `json:"official"`
	Source       string       `json:"source"`
	Status       string       `json:"status"`
	ChannelCount int          `json:"channel_count"`
	Deviation    *struct {
		Input *float64 `json:"input"`
	} `json:"deviation"`
}

type pricesDTO struct {
	Items  []priceRowDTO  `json:"items"`
	Counts map[string]int `json:"counts"`
}

func (m *m2) prices(query string) pricesDTO {
	m.t.Helper()
	var d pricesDTO
	m.must("GET", "/prices"+query, nil).into(m.t, &d)
	return d
}

func (d pricesDTO) find(model string) *priceRowDTO {
	for i := range d.Items {
		if d.Items[i].Model == model {
			return &d.Items[i]
		}
	}
	return nil
}

// Plan §11.1 M3 steps 1–4: price book → auto discount → manual survives → balance & unit cost.
func TestM3PricingAndBalance(t *testing.T) {
	m := newM3(t)
	_, ch := m.setupChannels()

	// step 1: price-book sync from the LiteLLM httptest server
	m.must("POST", "/jobs/price_book?wait=1", nil)
	if msg := m.jobMessage("price_book"); msg != "获取 1 条，更新 1 个，缺价 1 个" {
		t.Fatalf("price_book message: %q", msg)
	}
	d := m.prices("")
	row := d.find("test-model")
	if row == nil || row.Official == nil || row.Source != "litellm" || row.ChannelCount != 3 {
		t.Fatalf("test-model row: %+v", row)
	}
	o := row.Official
	if !near(*o.Input, 3, 1e-9) || !near(*o.Output, 15, 1e-9) || !near(*o.CacheRead, 0.3, 1e-9) || !near(*o.CacheWrite5m, 3.75, 1e-9) {
		t.Fatalf("official price: %+v", o)
	}

	// step 2: ratio job derives channel a = 1.5 × 2 × 0.4 ÷ 3 = 0.4 (auto)
	m.must("POST", "/jobs/ratio?wait=1", nil)
	if r := m.row(ch["a"]); r.DiscountSource != "auto" || !near(r.Discount, 0.4, 1e-6) {
		t.Fatalf("auto discount: %+v", r)
	}

	// step 3: manual 0.35 via draft → apply → ratio job again → still 0.35 manual
	m.must("PATCH", "/channels/drafts", map[string]any{"changes": []map[string]any{
		{"channel_id": ch["a"], "ext": map[string]any{"discount": 0.35}}}})
	m.must("POST", "/channels/apply", map[string]any{})
	m.must("POST", "/jobs/ratio?wait=1", nil)
	r := m.row(ch["a"])
	if r.DiscountSource != "manual" || !near(r.Discount, 0.35, 1e-9) {
		t.Fatalf("manual discount lost: %+v", r)
	}

	// step 4: balance 2500000 quota → 5.00 USD, ×0.5 = 2.50 CNY; unit cost 0.00996 × 0.35 × 0.5
	if r.BalanceUSD == nil || r.BalanceCNY == nil || !near(*r.BalanceUSD, 5, 1e-9) || !near(*r.BalanceCNY, 2.5, 1e-9) {
		t.Fatalf("balance: %+v", r)
	}
	if r.UnitCost == nil || !near(*r.UnitCost, 0.001743, 1e-6) || r.UnitCostModel != "test-model" {
		t.Fatalf("unit_cost: %+v", r)
	}
	if m.fake.Writes == 0 {
		t.Fatal("apply should have written NewAPI")
	}
}

func TestM3PricesStatusOverrideAlias(t *testing.T) {
	m := newM3(t)
	m.setupChannels()
	m.must("POST", "/prices/sync", nil)

	// no option set → normal; the vendor-only model has no price
	d := m.prices("")
	if tm := d.find("test-model"); tm == nil || tm.Status != "normal" {
		t.Fatalf("test-model: %+v", tm)
	}
	if my := d.find("mystery"); my == nil || my.Status != "no_price" || my.Official != nil {
		t.Fatalf("mystery: %+v", my)
	}
	if d.Counts["all"] != 2 || d.Counts["normal"] != 1 || d.Counts["no_price"] != 1 {
		t.Fatalf("counts: %+v", d.Counts)
	}
	if d.Items[0].Model != "mystery" { // unpriced first
		t.Fatalf("sort: %+v", d.Items)
	}

	// NewAPI ModelRatio 1.5 → input 3 (equal); CompletionRatio absent → unknown, still normal
	if err := m.fake.Ports().Options.Set(t.Context(), "ModelRatio", `{"test-model":1.5}`); err != nil {
		t.Fatal(err)
	}
	if tm := m.prices("").find("test-model"); tm.Status != "normal" {
		t.Fatalf("equal ratio: %+v", tm)
	}
	// 1.8 → +20% input → deviation
	if err := m.fake.Ports().Options.Set(t.Context(), "ModelRatio", `{"test-model":1.8}`); err != nil {
		t.Fatal(err)
	}
	dv := m.prices("?status=deviation")
	if len(dv.Items) != 1 || dv.Items[0].Model != "test-model" || dv.Items[0].Deviation == nil ||
		dv.Items[0].Deviation.Input == nil || !near(*dv.Items[0].Deviation.Input, 0.2, 1e-9) || dv.Counts["deviation"] != 1 {
		t.Fatalf("deviation: %+v counts %+v", dv.Items, dv.Counts)
	}
	if n := len(m.prices("?q=mys").Items); n != 1 {
		t.Fatalf("q filter: %d", n)
	}

	// override for the unpriced model → priced, source override
	if r := m.call("PUT", "/prices/mystery/override", map[string]any{"output": 5}); r.Code != 400 {
		t.Fatalf("input required: %d", r.Code)
	}
	if r := m.call("PUT", "/prices/mystery/override", map[string]any{"input": -1}); r.Code != 400 {
		t.Fatalf("negative: %d", r.Code)
	}
	m.must("PUT", "/prices/mystery/override", map[string]any{"input": 2, "output": 8, "remark": "手动"})
	my := m.prices("").find("mystery")
	if my.Official == nil || my.Source != "override" || my.Status == "no_price" {
		t.Fatalf("override: %+v", my)
	}
	m.must("DELETE", "/prices/mystery/override", nil)
	if my := m.prices("").find("mystery"); my.Status != "no_price" {
		t.Fatalf("override cleared: %+v", my)
	}

	// aliases
	m.must("PUT", "/aliases/test-model", map[string]any{"aliases": []string{"tm-latest", "tm-x"}})
	if r := m.call("PUT", "/aliases/other-model", map[string]any{"aliases": []string{"tm-x"}}); r.Code != 409 {
		t.Fatalf("alias of another official: %d %s", r.Code, r.Raw)
	}
	if r := m.call("PUT", "/aliases/other-model", map[string]any{"aliases": []string{"test-model"}}); r.Code != 400 {
		t.Fatalf("alias equals official: %d %s", r.Code, r.Raw)
	}
	if r := m.must("GET", "/aliases", nil); !strings.Contains(r.Raw, "tm-latest") {
		t.Fatalf("aliases: %s", r.Raw)
	}
	var det struct {
		Aliases  []string `json:"aliases"`
		Channels []struct {
			ID int `json:"id"`
		} `json:"channels"`
	}
	m.must("GET", "/prices/tm-latest", nil).into(t, &det)
	if len(det.Aliases) != 2 || len(det.Channels) != 3 {
		t.Fatalf("detail via alias: %+v", det)
	}
}

func TestM3ModelDiscountAndCacheOverride(t *testing.T) {
	m := newM3(t)
	up, ch := m.setupChannels()
	m.must("POST", "/prices/sync", nil)
	m.must("POST", "/jobs/ratio?wait=1", nil)
	base := "/channels/" + strconv.Itoa(ch["a"])

	type modelsDTO struct {
		Models []struct {
			Model     string   `json:"model"`
			Override  *float64 `json:"override"`
			Effective float64  `json:"effective"`
		} `json:"models"`
	}
	var md modelsDTO
	m.must("GET", base+"/models", nil).into(t, &md)
	if len(md.Models) != 1 || md.Models[0].Override != nil || !near(md.Models[0].Effective, 0.4, 1e-6) {
		t.Fatalf("models before: %+v", md)
	}
	before := m.auditTotal("channel")
	for _, bad := range []any{0, -1} {
		if r := m.call("PUT", base+"/model-discount", map[string]any{"model": "test-model", "discount": bad}); r.Code != 400 {
			t.Fatalf("discount %v: %d", bad, r.Code)
		}
	}
	m.must("PUT", base+"/model-discount", map[string]any{"model": "test-model", "discount": 0.3})
	md = modelsDTO{}
	m.must("GET", base+"/models", nil).into(t, &md)
	if md.Models[0].Override == nil || !near(*md.Models[0].Override, 0.3, 1e-9) || !near(md.Models[0].Effective, 0.3, 1e-9) {
		t.Fatalf("models after: %+v", md)
	}
	if got := m.auditTotal("channel") - before; got != 1 {
		t.Fatalf("model_discount audit rows: %d", got)
	}
	writes := m.fake.Writes
	m.must("PUT", base+"/model-discount", map[string]any{"model": "test-model", "discount": nil})
	md = modelsDTO{}
	m.must("GET", base+"/models", nil).into(t, &md)
	if md.Models[0].Override != nil {
		t.Fatalf("override not cleared: %+v", md)
	}

	// cache override: read fraction changes unit cost (200 cached tokens × 3 × 0.5 instead of 0.3)
	m.must("PUT", base+"/cache-override", map[string]any{"read": 0.5})
	if r := m.call("PUT", base+"/cache-override", map[string]any{"read": -1}); r.Code != 400 {
		t.Fatalf("negative cache: %d", r.Code)
	}
	want := (800*3 + 200*3*0.5 + 500*15) / 1e6 * 0.4 * 0.5
	if r := m.row(ch["a"]); r.UnitCost == nil || !near(*r.UnitCost, want, 1e-6) {
		t.Fatalf("unit cost with cache override: %+v want %v", r, want)
	}
	m.must("PUT", base+"/cache-override", map[string]any{})

	// unmanaged / unknown channel
	if r := m.call("PUT", "/channels/999/model-discount", map[string]any{"model": "x", "discount": 0.5}); r.Code != 400 {
		t.Fatalf("unmanaged: %d", r.Code)
	}

	// derive preview writes nothing
	var dv struct {
		Channels []struct {
			ChannelID  int `json:"channel_id"`
			Derivation struct {
				Median *float64 `json:"median"`
			} `json:"derivation"`
		} `json:"channels"`
	}
	m.must("POST", "/upstreams/"+strconv.Itoa(int(up))+"/derive", nil).into(t, &dv)
	if len(dv.Channels) != 3 {
		t.Fatalf("derive: %+v", dv)
	}
	for _, c := range dv.Channels { // group c has no upstream price: only a (0.4) and b (0.5) derive
		if c.ChannelID == ch["a"] && (c.Derivation.Median == nil || !near(*c.Derivation.Median, 0.4, 1e-6)) {
			t.Fatalf("derive a: %+v", dv)
		}
		if c.ChannelID == ch["c"] && c.Derivation.Median != nil {
			t.Fatalf("derive c should be underived: %+v", dv)
		}
	}
	if m.fake.Writes != writes {
		t.Fatalf("nam-only endpoints wrote NewAPI: %d → %d", writes, m.fake.Writes)
	}
}
