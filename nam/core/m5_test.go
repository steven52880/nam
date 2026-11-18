package core_test

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/steven52880/nam/ports"
)

type vpRef struct {
	Cost     float64 `json:"cost"`
	Priority int64   `json:"priority"`
	Channels []struct {
		ID          int     `json:"id"`
		Name        string  `json:"name"`
		Weight      uint    `json:"weight"`
		ChannelCost float64 `json:"channel_cost"`
	} `json:"channels"`
}

type vpItem struct {
	VendorID         uint     `json:"vendor_id"`
	Vendor           string   `json:"vendor"`
	Group            string   `json:"group"`
	Mode             string   `json:"mode"`
	Markup           *float64 `json:"markup"`
	SellPrice        *float64 `json:"sell_price"`
	Reference        *vpRef   `json:"reference"`
	TargetPrice      *float64 `json:"target_price"`
	TargetRatio      *float64 `json:"target_ratio"`
	NewAPIRatio      *float64 `json:"newapi_ratio"`
	LastWrittenPrice *float64 `json:"last_written_price"`
	LastWrittenRatio *float64 `json:"last_written_ratio"`
	PendingPrice     *float64 `json:"pending_price"`
	Change           *float64 `json:"change"`
	SmallChange      bool     `json:"small_change"`
	State            string   `json:"state"`
}

type vpOverview struct {
	SiteRate         float64  `json:"site_rate"`
	USDRate          float64  `json:"usd_rate"`
	AutoWrite        bool     `json:"auto_write"`
	ProtectThreshold float64  `json:"protect_threshold"`
	MinChange        float64  `json:"min_change"`
	Items            []vpItem `json:"items"`
	Groups           struct {
		AutoGroups struct {
			Current []string `json:"current"`
			Target  []string `json:"target"`
		} `json:"auto_groups"`
		UserUsableGroups struct {
			Current map[string]string `json:"current"`
			Target  map[string]string `json:"target"`
		} `json:"user_usable_groups"`
		Changed bool `json:"changed"`
	} `json:"groups"`
	Error string `json:"error"`
}

type vpApply struct {
	Written []string `json:"written"`
	Vendors []struct {
		VendorID    uint     `json:"vendor_id"`
		BeforeRatio *float64 `json:"before_ratio"`
		AfterRatio  *float64 `json:"after_ratio"`
	} `json:"vendors"`
	Skipped []struct {
		VendorID uint   `json:"vendor_id"`
		State    string `json:"state"`
		Reason   string `json:"reason"`
	} `json:"skipped"`
	Options []struct {
		Key    string `json:"key"`
		Before string `json:"before"`
		After  string `json:"after"`
	} `json:"options"`
}

func (m *m2) setOpt(k, v string) {
	m.t.Helper()
	if err := m.fake.Ports().Options.Set(context.Background(), k, v); err != nil {
		m.t.Fatal(err)
	}
}

func (m *m2) opt(k string) string {
	m.t.Helper()
	got, err := m.fake.Ports().Options.Get(context.Background(), k)
	if err != nil {
		m.t.Fatal(err)
	}
	return got[k]
}

func (m *m2) groupRatio(group string) float64 {
	m.t.Helper()
	var gr map[string]float64
	if err := json.Unmarshal([]byte(m.opt("GroupRatio")), &gr); err != nil {
		m.t.Fatalf("GroupRatio %q: %v", m.opt("GroupRatio"), err)
	}
	return gr[group]
}

func (m *m2) vp() vpOverview {
	m.t.Helper()
	var ov vpOverview
	m.must("GET", "/vendor-pricing", nil).into(m.t, &ov)
	return ov
}

func (m *m2) vpItem() vpItem {
	m.t.Helper()
	ov := m.vp()
	if len(ov.Items) != 1 {
		m.t.Fatalf("items: %+v", ov.Items)
	}
	return ov.Items[0]
}

func (m *m2) putVP(id uint, body map[string]any) vpItem {
	m.t.Helper()
	var it vpItem
	m.must("PUT", "/vendor-pricing/"+itoa(id), body).into(m.t, &it)
	return it
}

func itoa(id uint) string { return strconv.FormatUint(uint64(id), 10) }

func (m *m2) pricingJob() string {
	m.t.Helper()
	m.must("POST", "/jobs/pricing?wait=1", nil)
	return m.jobMessage("pricing")
}

func wantNear(t *testing.T, what string, got *float64, want float64) {
	t.Helper()
	if got == nil || !near(*got, want, 1e-9) {
		t.Fatalf("%s: got %v, want %v", what, got, want)
	}
}

// m5Fixture: vendor anthropic with channels a (enabled), b and c (manually disabled), every
// channel with discount 0.35 on an upstream with currency rate 0.5, so the 上游渠道售价 of each
// is 0.35 × 0.5 ÷ 7.2. NewAPI starts with default group options and model prices nam must not touch.
func m5Fixture(t *testing.T) *m2 {
	t.Helper()
	m := newM3(t)
	_, ch := m.setupChannels()
	m.must("PATCH", "/channels/drafts", map[string]any{"changes": []map[string]any{
		{"channel_id": ch["a"], "ext": map[string]any{"discount": 0.35}},
		{"channel_id": ch["b"], "ext": map[string]any{"discount": 0.35}},
		{"channel_id": ch["c"], "ext": map[string]any{"discount": 0.35}}}})
	m.must("POST", "/channels/apply", map[string]any{})
	disabled := ports.ChannelManualDisabled
	for _, l := range []string{"b", "c"} {
		if err := m.fake.Update(context.Background(), ch[l], ports.ChannelPatch{Status: &disabled}); err != nil {
			t.Fatal(err)
		}
	}
	m.setOpt("GroupRatio", `{"default":1}`)
	m.setOpt("AutoGroups", `["default"]`)
	m.setOpt("UserUsableGroups", `{"default":"默认"}`)
	m.setOpt("ModelRatio", `{"test-model":1.5}`)
	m.setOpt("CompletionRatio", `{"test-model":5}`)
	m.setOpt("CacheRatio", `{"test-model":0.1}`)
	m.setOpt("CreateCacheRatio", `{"test-model":1.25}`)
	return m
}

func (m *m2) modelOptionsUntouched() {
	m.t.Helper()
	for k, v := range map[string]string{
		"ModelRatio": `{"test-model":1.5}`, "CompletionRatio": `{"test-model":5}`,
		"CacheRatio": `{"test-model":0.1}`, "CreateCacheRatio": `{"test-model":1.25}`,
	} {
		if got := m.opt(k); got != v {
			m.t.Fatalf("%s must not be written by pricing: %s", k, got)
		}
	}
}

func TestM5FixedAndFollowFlow(t *testing.T) {
	m := m5Fixture(t)
	cost := 0.35 * 0.5 / 7.2

	// overview: only channel a counts, vendor is unset (no GroupRatio[anthropic] yet)
	ov := m.vp()
	if ov.SiteRate != 7 || ov.USDRate != 7.2 || ov.AutoWrite || ov.ProtectThreshold != 0.5 || ov.MinChange != 0.02 || ov.Error != "" {
		t.Fatalf("overview header: %+v", ov)
	}
	it := m.vpItem()
	if it.Vendor != "anthropic" || it.Group != "anthropic" || it.State != "unset" || it.Mode != "fixed" || it.SellPrice != nil {
		t.Fatalf("initial item: %+v", it)
	}
	if it.Reference == nil || len(it.Reference.Channels) != 1 || !near(it.Reference.Cost, cost, 1e-9) || it.Reference.Channels[0].Weight != 1 {
		t.Fatalf("reference: %+v", it.Reference)
	}
	if !ov.Groups.Changed || strings.Join(ov.Groups.AutoGroups.Target, ",") != "anthropic,default" || ov.Groups.UserUsableGroups.Target["auto"] != "自动选择" {
		t.Fatalf("groups: %+v", ov.Groups)
	}
	id := it.VendorID

	// ── 固定模式：售价 0.35 → GroupRatio = 0.35 × 7.2 ÷ 7 = 0.36
	it = m.putVP(id, map[string]any{"mode": "fixed", "sell_price": 0.35})
	if it.State != "to_write" {
		t.Fatalf("after PUT: %+v", it)
	}
	wantNear(t, "target ratio", it.TargetRatio, 0.36)
	if msg := m.pricingJob(); !strings.Contains(msg, "写入 0") || m.groupRatio("anthropic") != 0 {
		t.Fatalf("the job never writes a fixed-mode vendor: %q %s", msg, m.opt("GroupRatio"))
	}
	var pv vpApply
	writes := m.fake.Writes
	m.must("POST", "/vendor-pricing/preview", map[string]any{}).into(t, &pv)
	if len(pv.Vendors) != 1 || len(pv.Options) != 3 || pv.Options[0].Key != "GroupRatio" || m.fake.Writes != writes {
		t.Fatalf("preview: %+v", pv)
	}
	wantNear(t, "preview after_ratio", pv.Vendors[0].AfterRatio, 0.36)

	priceAudits := m.auditTotal("price")
	var ap vpApply
	m.must("POST", "/vendor-pricing/apply", map[string]any{}).into(t, &ap)
	if strings.Join(ap.Written, ",") != "GroupRatio,UserUsableGroups,AutoGroups" || len(ap.Vendors) != 1 {
		t.Fatalf("apply: %+v", ap)
	}
	if got := m.groupRatio("anthropic"); got != 0.36 || m.groupRatio("default") != 1 {
		t.Fatalf("GroupRatio: %s", m.opt("GroupRatio"))
	}
	if m.opt("AutoGroups") != `["anthropic","default"]` {
		t.Fatalf("AutoGroups: %s", m.opt("AutoGroups"))
	}
	var uug map[string]string
	_ = json.Unmarshal([]byte(m.opt("UserUsableGroups")), &uug)
	if uug["default"] != "默认" || uug["anthropic"] != "anthropic" || uug["auto"] != "自动选择" || len(uug) != 3 {
		t.Fatalf("UserUsableGroups: %v", uug)
	}
	m.modelOptionsUntouched()
	if after := m.auditTotal("price"); after != priceAudits+1 {
		t.Fatalf("one GroupRatio audit row expected: %d → %d", priceAudits, after)
	}
	if it = m.vpItem(); it.State != "synced" || !near(*it.LastWrittenRatio, 0.36, 1e-9) {
		t.Fatalf("after apply: %+v", it)
	}

	// ── 跟随模式：0.0243 × 1.5 → 0.0365，比上次的 0.35 低 89.6%，超过阈值 → 待确认
	it = m.putVP(id, map[string]any{"mode": "follow", "markup": 1.5})
	wantNear(t, "target price", it.TargetPrice, 0.0365)
	if it.State != "pending" || it.Change == nil || *it.Change < 0.89 || it.PendingPrice == nil {
		t.Fatalf("pending: %+v", it)
	}
	m.must("PUT", "/settings", map[string]string{"pricing_auto_write": "true"})
	if msg := m.pricingJob(); !strings.Contains(msg, "待确认 1") || !strings.Contains(msg, "写入 0") || m.groupRatio("anthropic") != 0.36 {
		t.Fatalf("job must leave a pending vendor alone: %q", msg)
	}
	m.must("POST", "/vendor-pricing/apply", map[string]any{"include_groups": false}).into(t, &ap)
	if len(ap.Written) != 0 || len(ap.Skipped) != 1 || ap.Skipped[0].State != "pending" || !strings.Contains(ap.Skipped[0].Reason, "变化保护阈值") {
		t.Fatalf("apply skips pending: %+v", ap)
	}
	m.must("POST", "/vendor-pricing/"+itoa(id)+"/confirm", nil).into(t, &ap)
	if len(ap.Written) != 1 || ap.Written[0] != "GroupRatio" || m.groupRatio("anthropic") != 0.038 {
		t.Fatalf("confirm: %+v ratio %v", ap, m.groupRatio("anthropic"))
	}
	if it = m.vpItem(); it.State != "synced" || it.PendingPrice != nil {
		t.Fatalf("after confirm: %+v", it)
	}
	if r := m.call("POST", "/vendor-pricing/"+itoa(id)+"/confirm", nil); r.Code != 409 {
		t.Fatalf("confirm on a synced vendor: %d %s", r.Code, r.Raw)
	}

	// ── 正常变化（+6.6%）：自动写入已开启，定价任务直接写
	m.putVP(id, map[string]any{"markup": 1.6})
	if msg := m.pricingJob(); !strings.Contains(msg, "写入 1") || m.groupRatio("anthropic") != 0.040 { // 0.0389 × 7.2 ÷ 7 = 0.040011
		t.Fatalf("auto write: %q ratio %v", msg, m.groupRatio("anthropic"))
	}
	// ── 小变化（+1.3% < 2%）：任务不写，手动预览仍包含
	it = m.putVP(id, map[string]any{"markup": 1.62})
	if it.State != "to_write" || !it.SmallChange {
		t.Fatalf("small change: %+v", it)
	}
	if msg := m.pricingJob(); !strings.Contains(msg, "写入 0") || m.groupRatio("anthropic") != 0.040 {
		t.Fatalf("small change must not be auto written: %q", msg)
	}
	m.must("POST", "/vendor-pricing/preview", map[string]any{}).into(t, &pv)
	if len(pv.Vendors) != 1 {
		t.Fatalf("preview includes small changes: %+v", pv)
	}
	m.modelOptionsUntouched()
}

func TestM5DriftForceAdoptAndSafety(t *testing.T) {
	m := m5Fixture(t)
	id := m.vpItem().VendorID
	m.putVP(id, map[string]any{"mode": "fixed", "sell_price": 0.35})
	m.must("POST", "/vendor-pricing/apply", map[string]any{})
	m.must("PUT", "/settings", map[string]string{"pricing_auto_write": "true"})

	// 有人在 NewAPI 里手动改了倍率 → 漂移，自动写入跳过
	m.setOpt("GroupRatio", `{"default":1,"anthropic":0.9}`)
	if it := m.vpItem(); it.State != "drift" {
		t.Fatalf("drift: %+v", it)
	}
	if msg := m.pricingJob(); !strings.Contains(msg, "漂移 1") || m.groupRatio("anthropic") != 0.9 {
		t.Fatalf("job: %q", msg)
	}
	var ap vpApply
	writes := m.fake.Writes
	m.must("POST", "/vendor-pricing/apply", map[string]any{"vendor_ids": []uint{id}, "include_groups": false}).into(t, &ap)
	if len(ap.Skipped) != 1 || ap.Skipped[0].State != "drift" || m.fake.Writes != writes {
		t.Fatalf("drift without force: %+v", ap)
	}
	// 以 nam 为准覆盖
	m.must("POST", "/vendor-pricing/apply", map[string]any{"vendor_ids": []uint{id}, "force": true}).into(t, &ap)
	if len(ap.Vendors) != 1 || m.groupRatio("anthropic") != 0.36 {
		t.Fatalf("force: %+v ratio %v", ap, m.groupRatio("anthropic"))
	}
	// 采纳 NewAPI 值
	m.setOpt("GroupRatio", `{"default":1,"anthropic":0.9}`)
	writes = m.fake.Writes
	var it vpItem
	m.must("POST", "/vendor-pricing/"+itoa(id)+"/adopt", nil).into(t, &it)
	wantNear(t, "adopted price", it.SellPrice, 0.9*7/7.2)
	if it.State != "synced" || it.Mode != "fixed" || m.fake.Writes != writes || m.groupRatio("anthropic") != 0.9 {
		t.Fatalf("adopt: %+v", it)
	}
	if r := m.call("POST", "/vendor-pricing/"+itoa(id)+"/adopt", nil); r.Code != 409 {
		t.Fatalf("adopt on a synced vendor: %d", r.Code)
	}

	// 选项不是合法 JSON：报告错误，绝不写入
	m.putVP(id, map[string]any{"sell_price": 0.4})
	m.setOpt("AutoGroups", `{broken`)
	if ov := m.vp(); ov.Error == "" || !strings.Contains(ov.Error, "AutoGroups") {
		t.Fatalf("error: %q", ov.Error)
	}
	writes = m.fake.Writes
	for _, p := range []string{"/vendor-pricing/apply", "/vendor-pricing/preview"} {
		if r := m.call("POST", p, map[string]any{}); r.Code != 409 {
			t.Fatalf("%s with a broken option: %d %s", p, r.Code, r.Raw)
		}
	}
	if m.fake.Writes != writes || m.opt("AutoGroups") != `{broken` {
		t.Fatal("a broken option must be left alone")
	}

	// 参数校验
	for name, body := range map[string]map[string]any{
		"price 0":    {"sell_price": 0},
		"price 11":   {"sell_price": 11},
		"markup 0":   {"markup": 0},
		"markup 101": {"markup": 101},
		"bad mode":   {"mode": "x"},
	} {
		if r := m.call("PUT", "/vendor-pricing/"+itoa(id), body); r.Code != 400 {
			t.Errorf("%s: %d %s", name, r.Code, r.Raw)
		}
	}
	if r := m.call("PUT", "/vendor-pricing/999", map[string]any{"sell_price": 1}); r.Code != 404 {
		t.Errorf("unknown vendor: %d", r.Code)
	}
	if w := do(m.h, "GET", "/api/nam/vendor-pricing", "", ""); w.Code != 401 {
		t.Errorf("anonymous: %d", w.Code)
	}
	// 空请求体 = 全部默认
	if w := do(m.h, "POST", "/api/nam/vendor-pricing/preview", "", "100"); w.Code != 409 { // AutoGroups is still broken
		t.Errorf("empty body: %d %s", w.Code, w.Body)
	}
}

func TestM5EditGroupRatioDirectly(t *testing.T) {
	m := m5Fixture(t)
	id := m.vpItem().VendorID

	// 固定模式（新厂商默认）：直接改倍率，目标倍率恰好等于输入（舍入到 3 位）
	it := m.putVP(id, map[string]any{"group_ratio": 0.3101})
	if it.Mode != "fixed" || it.State != "to_write" || it.PendingPrice != nil {
		t.Fatalf("after PUT group_ratio: %+v", it)
	}
	wantNear(t, "target ratio", it.TargetRatio, 0.31)
	wantNear(t, "sell price", it.SellPrice, 0.31*7/7.2)
	writes := m.fake.Writes
	if m.groupRatio("anthropic") != 0 || m.fake.Writes != writes {
		t.Fatal("editing the ratio writes nam only")
	}
	m.must("POST", "/vendor-pricing/apply", map[string]any{"include_groups": false})
	if got := m.groupRatio("anthropic"); got != 0.31 {
		t.Fatalf("written ratio: %v", got)
	}

	// 跟随模式：400，说明原因
	m.putVP(id, map[string]any{"mode": "follow", "markup": 1.5})
	for name, body := range map[string]map[string]any{
		"follow mode":         {"group_ratio": 0.5},
		"switching to follow": {"mode": "follow", "group_ratio": 0.5},
	} {
		r := m.call("PUT", "/vendor-pricing/"+itoa(id), body)
		if r.Code != 400 || !strings.Contains(string(r.Raw), "固定模式") {
			t.Errorf("%s: %d %s", name, r.Code, r.Raw)
		}
	}
	if it = m.vpItem(); it.Mode != "follow" {
		t.Fatalf("a rejected edit must not change the vendor: %+v", it)
	}
	// 同一请求里改回固定模式并给倍率：可以
	it = m.putVP(id, map[string]any{"mode": "fixed", "group_ratio": 0.5})
	wantNear(t, "target ratio", it.TargetRatio, 0.5)

	// 其它非法输入
	for name, body := range map[string]map[string]any{
		"both fields":    {"group_ratio": 0.5, "sell_price": 0.3},
		"zero":           {"group_ratio": 0},
		"negative":       {"group_ratio": -1},
		"rounds to zero": {"group_ratio": 0.0004},
		"price > 10":     {"group_ratio": 11},
	} {
		if r := m.call("PUT", "/vendor-pricing/"+itoa(id), body); r.Code != 400 {
			t.Errorf("%s: %d %s", name, r.Code, r.Raw)
		}
	}
	wantNear(t, "ratio unchanged after bad edits", m.vpItem().TargetRatio, 0.5)
}

func TestM5TakeoverKeepsPriceAndUSDRateChange(t *testing.T) {
	m := newM2(t)
	m.must("POST", "/vendors", map[string]any{"name": "anthropic", "group_name": "anthropic", "models": []string{"test-model"}})
	m.setOpt("GroupRatio", `{"anthropic":0.2857,"default":1}`)
	writes := m.fake.Writes

	it := m.vpItem()
	wantNear(t, "taken-over price", it.SellPrice, 0.2857*7/7.2)
	if it.Mode != "fixed" || it.State != "synced" || it.NewAPIRatio == nil || *it.NewAPIRatio != 0.2857 || m.fake.Writes != writes {
		t.Fatalf("takeover must keep the price and write nothing: %+v", it)
	}
	wantNear(t, "target ratio", it.TargetRatio, 0.286) // 0.2857 rounded to 3 decimals; still synced

	// 改美元参考汇率：售价不变，目标倍率变了，固定模式也要手动写入
	m.must("PUT", "/settings", map[string]string{"usd_cny_rate": "7.5"})
	it = m.vpItem()
	if it.State != "to_write" || m.fake.Writes != writes {
		t.Fatalf("after usd rate change: %+v", it)
	}
	wantNear(t, "sell price unchanged", it.SellPrice, 0.2857*7/7.2)
	wantNear(t, "new target ratio", it.TargetRatio, 0.298)
	m.must("POST", "/vendor-pricing/apply", map[string]any{"include_groups": false})
	if got := m.groupRatio("anthropic"); got != 0.298 {
		t.Fatalf("ratio: %v", got)
	}
}

func TestM5SettingsAndJob(t *testing.T) {
	m := newM2(t)
	var s map[string]string
	m.must("GET", "/settings", nil).into(t, &s)
	if _, old := s["auto_write"]; old || s["pricing_auto_write"] != "false" || s["pricing_min_change"] != "0.02" {
		t.Fatalf("settings: %v", s)
	}
	for k, v := range map[string]string{
		"auto_write":         "true",
		"pricing_auto_write": "yes",
		"pricing_min_change": "1",
	} {
		if r := m.call("PUT", "/settings", map[string]string{k: v}); r.Code != 400 {
			t.Errorf("%s=%s: %d", k, v, r.Code)
		}
	}
	if r := m.call("PUT", "/settings", map[string]string{"pricing_min_change": "-0.1"}); r.Code != 400 {
		t.Errorf("negative min change: %d", r.Code)
	}
	m.must("PUT", "/settings", map[string]string{"pricing_min_change": "0", "pricing_auto_write": "true"})
	m.must("GET", "/settings", nil).into(t, &s)
	if s["pricing_min_change"] != "0" || s["pricing_auto_write"] != "true" {
		t.Fatalf("settings after PUT: %v", s)
	}
	var js []struct {
		Name      string `json:"name"`
		IntervalS int64  `json:"interval_s"`
	}
	m.must("GET", "/jobs", nil).into(t, &js)
	for _, j := range js {
		if j.Name == "pricing" && j.IntervalS != 3600 {
			t.Fatalf("pricing job interval: %d", j.IntervalS)
		}
	}
	if msg := m.pricingJob(); msg != "0 个厂商；写入 0；待确认 0；漂移 0；无可用渠道 0" {
		t.Fatalf("empty job: %q", msg)
	}
}
