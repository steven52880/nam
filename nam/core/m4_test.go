package core_test

import (
	"context"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/steven52880/nam/internal/stats"
	"github.com/steven52880/nam/ports"
)

// m4Fixture: upstream (rate 0.5) + vendor anthropic + channels a,b with manual discount 0.35,
// price book loaded, then the plan's logs. Returns channel ids of a and b and the base time.
func m4Fixture(t *testing.T) (*m2, int, int, int64) {
	t.Helper()
	m := newM3(t)
	_, ch := m.setupChannels()
	m.must("POST", "/jobs/price_book?wait=1", nil)
	m.must("PATCH", "/channels/drafts", map[string]any{"changes": []map[string]any{
		{"channel_id": ch["a"], "ext": map[string]any{"discount": 0.35}},
		{"channel_id": ch["b"], "ext": map[string]any{"discount": 0.35}}}})
	m.must("POST", "/channels/apply", map[string]any{})

	base := time.Now().Unix()
	base = base - base%3600 - 1800 // :30 of the previous hour: every log lands in one hour
	consume := func(chID int, at int64, other string, useTime int, token string) ports.LogRecord {
		return ports.LogRecord{ChannelID: chID, CreatedAt: at, Type: ports.LogTypeConsume, ModelName: "test-model", Group: "anthropic",
			PromptTokens: 1000, CompletionTokens: 500, IsStream: true, Quota: 1743, UseTime: useTime, Other: other, TokenName: token}
	}
	for range 10 {
		m.fake.AddLog(consume(ch["a"], base, `{"cache_tokens":200,"frt":300,"use_time_ms":2300}`, 3, "tk"))
	}
	for range 2 {
		m.fake.AddLog(ports.LogRecord{ChannelID: ch["a"], CreatedAt: base - 60, Type: ports.LogTypeError, ModelName: "test-model",
			Group: "anthropic", Content: "upstream error", Other: `{"status_code":500}`, TokenName: "tk"})
	}
	m.fake.AddLog(consume(ch["a"], base, `{"cache_tokens":200,"frt":300,"use_time_ms":2300}`, 3, stats.TestTokenName))
	for range 5 {
		m.fake.AddLog(consume(ch["b"], base, `{"cache_tokens":200,"frt":300}`, 2, "tk"))
	}
	m.must("POST", "/jobs/pull_logs?wait=1", nil)
	return m, ch["a"], ch["b"], base
}

func (m *m2) chStats(window string) map[int]stats.ChannelStatsItem {
	m.t.Helper()
	var d stats.ChannelStatsList
	m.must("GET", "/stats/channels?window="+window, nil).into(m.t, &d)
	out := map[int]stats.ChannelStatsItem{}
	for _, it := range d.Items {
		out[it.ChannelID] = it
	}
	return out
}

func f(p *float64) float64 {
	if p == nil {
		return math.NaN()
	}
	return *p
}

func TestM4StatsQueries(t *testing.T) {
	m, a, b, base := m4Fixture(t)
	now := time.Now().Unix()
	q := "from=" + strconv.FormatInt(now-86400, 10) + "&to=" + strconv.FormatInt(now+3600, 10)

	s3 := m.chStats("3d")
	ca, cb := s3[a], s3[b]
	if ca.Success != 10 || ca.Fail != 2 || ca.FailByClass["5xx"] != 2 || len(ca.FailByClass) != 1 {
		t.Fatalf("channel a counts: %+v", ca)
	}
	if !near(f(ca.CacheHit), 0.2, 1e-9) || !near(f(ca.EffectiveCostPrice), 0.001743/0.0105, 1e-6) || !near(f(ca.MeasuredCostPrice), 0.175, 1e-6) {
		t.Fatalf("channel a cache/cost: hit=%v eff=%v meas=%v", ca.CacheHit, ca.EffectiveCostPrice, ca.MeasuredCostPrice)
	}
	if ca.SpeedEstimated || !near(f(ca.SpeedP50), 250, 0.05) {
		t.Fatalf("channel a speed: est=%v p50=%v", ca.SpeedEstimated, ca.SpeedP50)
	}
	if !cb.SpeedEstimated || f(cb.SpeedP50) < 290 || f(cb.SpeedP50) > 310 {
		t.Fatalf("channel b speed: est=%v p50=%v", cb.SpeedEstimated, cb.SpeedP50)
	}
	for _, c := range []stats.ChannelStatsItem{ca, cb} {
		if !near(f(c.StdCostPrice), 0.175, 1e-6) || c.PriceTier == nil {
			t.Fatalf("channel %d std/tier: std=%v tier=%v", c.ChannelID, c.StdCostPrice, c.PriceTier)
		}
	}
	if ca.QualityTier != "insufficient" {
		t.Fatalf("quality tier: %q", ca.QualityTier)
	}

	lat := m.chStats("latest")[a]
	if lat.Success != ca.Success || lat.Fail != ca.Fail {
		t.Fatalf("latest vs 3d: %d/%d vs %d/%d", lat.Success, lat.Fail, ca.Success, ca.Fail)
	}

	var models stats.ModelStatsList
	m.must("GET", "/stats/channels/"+strconv.Itoa(a)+"/models?window=7d", nil).into(t, &models)
	if len(models.Items) != 1 || models.Items[0].Model != "test-model" || models.Items[0].Success != 10 {
		t.Fatalf("models: %+v", models.Items)
	}

	var ov stats.OverviewResult
	m.must("GET", "/stats/overview?dim=vendor&"+q, nil).into(t, &ov)
	if len(ov.Rows) != 1 || ov.Rows[0].Key != "anthropic" || ov.Rows[0].Success != 15 {
		t.Fatalf("overview rows: %+v", ov.Rows)
	}
	if want := 15.0 * 1743 / 500000 * 7; !near(ov.Totals.RevenueCNY, want, 1e-9) {
		t.Fatalf("revenue %v want %v", ov.Totals.RevenueCNY, want)
	}
	if len(ov.ErrorClasses) != 1 || ov.ErrorClasses["5xx"] != 2 || len(ov.Scatter) != 2 {
		t.Fatalf("overview classes/scatter: %+v %+v", ov.ErrorClasses, ov.Scatter)
	}

	var se stats.SeriesResult
	m.must("GET", "/stats/series?bucket=hour&from="+strconv.FormatInt(base-7200, 10)+"&to="+strconv.FormatInt(base+7200, 10), nil).into(t, &se)
	if len(se.Points) != 1 || se.Points[0].Success != 15 {
		t.Fatalf("series: %+v", se.Points)
	}

	var st stats.Status
	m.must("GET", "/stats/status", nil).into(t, &st)
	if st.CursorID != 18 || st.Rows < 1 {
		t.Fatalf("status: %+v", st)
	}
	var be struct {
		PatchP1 bool `json:"patch_p1"`
	}
	m.must("GET", "/backend", nil).into(t, &be)
	if !be.PatchP1 {
		t.Fatal("backend patch_p1 should be true after pulling a use_time_ms log")
	}

	var hint stats.ChannelStatsList
	m.must("GET", "/stats/channels?window=3d", nil).into(t, &hint)
	if hint.ErrorLogHint {
		t.Fatal("error_log_hint should be false while error logs exist")
	}
	if _, err := m.fake.Ports().Logs.DeleteBefore(context.Background(), base); err != nil {
		t.Fatal(err)
	}
	m.must("POST", "/stats/rebuild", map[string]any{"days": 1})
	m.must("POST", "/jobs/pull_logs?wait=1", nil)
	m.must("GET", "/stats/channels?window=3d", nil).into(t, &hint)
	if !hint.ErrorLogHint {
		t.Fatal("error_log_hint should be true after the error logs are gone and stats rebuilt")
	}
}

func TestM4StatsBadParams(t *testing.T) {
	m := newM3(t)
	now := time.Now().Unix()
	ts := strconv.FormatInt
	cases := []struct{ name, method, path string }{
		{"bad window", "GET", "/stats/channels?window=x"},
		{"bad window models", "GET", "/stats/channels/1/models?window=x"},
		{"from>to", "GET", "/stats/overview?dim=channel&from=" + ts(now, 10) + "&to=" + ts(now-10, 10)},
		{"span>186d", "GET", "/stats/overview?dim=channel&from=" + ts(now-187*86400, 10) + "&to=" + ts(now, 10)},
		{"bad dim", "GET", "/stats/overview?dim=nope&from=" + ts(now-10, 10) + "&to=" + ts(now, 10)},
		{"non-integer from", "GET", "/stats/overview?dim=channel&from=abc&to=" + ts(now, 10)},
		{"bad bucket", "GET", "/stats/series?bucket=week&from=" + ts(now-10, 10) + "&to=" + ts(now, 10)},
		{"series span", "GET", "/stats/series?bucket=day&from=" + ts(now-187*86400, 10) + "&to=" + ts(now, 10)},
		{"int64 overflow", "GET", "/stats/overview?dim=channel&from=-9223372036854775808&to=9223372036854775807"},
		{"series int64 overflow", "GET", "/stats/series?bucket=hour&from=-9223372036854775808&to=9223372036854775807"},
	}
	for _, c := range cases {
		if r := m.call(c.method, c.path, nil); r.Code != 400 {
			t.Errorf("%s: want 400, got %d %s", c.name, r.Code, r.Raw)
		}
	}
	for _, days := range []int{0, 91} {
		if r := m.call("POST", "/stats/rebuild", map[string]any{"days": days}); r.Code != 400 {
			t.Errorf("rebuild days=%d: want 400, got %d", days, r.Code)
		}
	}
	for _, kv := range []map[string]string{{"stats_backfill_days": "0"}, {"stats_backfill_days": "91"}, {"newapi_log_cleanup": "yes"}} {
		if r := m.call("PUT", "/settings", kv); r.Code != 400 {
			t.Errorf("settings %v: want 400, got %d", kv, r.Code)
		}
	}
	if r := m.call("PUT", "/settings", map[string]string{"stats_backfill_days": "30", "newapi_log_cleanup": "true"}); r.Code != 200 {
		t.Errorf("valid settings: %d %s", r.Code, r.Raw)
	}
}

func TestM4EmptyListsAreArrays(t *testing.T) {
	m := newM3(t)
	now := time.Now().Unix()
	for _, p := range []string{
		"/stats/channels?window=latest", "/stats/channels?window=7d", "/stats/channels/1/models",
		"/stats/overview?dim=model&from=" + strconv.FormatInt(now-100, 10) + "&to=" + strconv.FormatInt(now, 10),
		"/stats/series?bucket=day&from=" + strconv.FormatInt(now-100, 10) + "&to=" + strconv.FormatInt(now, 10),
	} {
		r := m.must("GET", p, nil)
		for _, bad := range []string{`"items":null`, `"rows":null`, `"points":null`, `"scatter":null`} {
			if containsStr(r.Raw, bad) {
				t.Errorf("%s: contains %s: %s", p, bad, r.Raw)
			}
		}
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
