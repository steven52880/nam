package stats

import (
	"strings"
	"testing"
	"time"

	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

func TestPull_aggregates_and_is_idempotent(t *testing.T) {
	// Given 10 standard logs, 2 errors and 1 channel-test log on channel 1.
	e := newEnv(t)
	at := time.Now().Unix() - 3600
	for range 10 {
		e.fake.AddLog(stdLog(1, at))
	}
	e.fake.AddLog(errLog(1, at))
	e.fake.AddLog(errLog(1, at))
	test := stdLog(1, at)
	test.TokenName = TestTokenName
	e.fake.AddLog(test)

	// When pulling twice.
	msg, err := e.svc.Pull(e.ctx)
	must(t, err)
	_, err = e.svc.Pull(e.ctx)
	must(t, err)

	// Then totals are counted once and the cursor sits past the skipped test log.
	if ok, fail := e.totals(t); ok != 10 || fail != 2 {
		t.Fatalf("totals %d/%d", ok, fail)
	}
	if c := e.cursor(t); c.LastID != 13 || c.LastAt != at {
		t.Fatalf("cursor %+v", c)
	}
	if !strings.Contains(msg, "成功 10、失败 2、跳过 1") {
		t.Fatalf("message %q", msg)
	}
	var row store.StatsHourly
	must(t, e.st.DB.First(&row, "channel_id = 1").Error)
	m := FromRow(row).Metrics(1)
	if row.Vendor != "anthropic" || m.FailByClass[Class5xx] != 2 || !near(*m.MeasuredCostPrice, 0.175, 1e-9) ||
		!near(*m.CacheHit, 0.2, 1e-12) || m.PricedRequests != 10 || !near(m.CostCNY, 0.01743, 1e-9) {
		t.Fatalf("row %+v metrics %+v", row, m)
	}
}

func TestPull_marks_patch_and_error_log_seen(t *testing.T) {
	e := newEnv(t)
	at := time.Now().Unix() - 3600
	e.fake.AddLog(stdLog(1, at))
	e.fake.AddLog(errLog(1, at+5))
	_, err := e.svc.Pull(e.ctx)
	must(t, err)
	if !e.st.GetBool(e.ctx, store.KeyStatsPatchP1Seen, false) {
		t.Fatal("patch p1 not marked")
	}
	if v := e.st.GetFloat(e.ctx, store.KeyStatsErrorLogSeenAt, 0); int64(v) != at+5 {
		t.Fatalf("error seen at %v", v)
	}
	if hint, err := e.svc.ErrorLogHint(e.ctx); err != nil || hint {
		t.Fatalf("hint %v %v", hint, err)
	}
}

func TestPull_crosses_batch_boundary(t *testing.T) {
	e := newEnv(t)
	at := time.Now().Unix() - 3600
	const n = 1203
	for i := range n {
		r := stdLog(1+i%2, at)
		if i%100 == 0 {
			r.TokenName = TestTokenName
		}
		e.fake.AddLog(r)
	}
	_, err := e.svc.Pull(e.ctx)
	must(t, err)
	ok, _ := e.totals(t)
	if ok != n-13 { // ids 1,101,...,1201 are tests
		t.Fatalf("success %d", ok)
	}
	if c := e.cursor(t); c.LastID != n {
		t.Fatalf("cursor %+v", c)
	}
	var vendors []string
	must(t, e.st.DB.Model(&store.StatsHourly{}).Distinct().Pluck("vendor", &vendors).Error)
	if len(vendors) != 1 || vendors[0] != "anthropic" {
		t.Fatalf("vendors %v", vendors)
	}
}

func TestPull_vendor_falls_back_to_channel_groups_then_empty(t *testing.T) {
	e := newEnv(t)
	// channel 1 exists in NewAPI and carries the anthropic group: that is its vendor
	id, err := e.fake.Create(e.ctx, ports.ChannelCreate{Name: "c1", Group: "default,anthropic", Models: []string{"test-model"}, Status: 1})
	must(t, err)
	if id != 1 {
		t.Fatalf("channel id %d", id)
	}
	at := time.Now().Unix() - 3600
	a := stdLog(1, at)
	a.Group = "default"
	b := stdLog(9, at)
	b.Group = "default"
	e.fake.AddLog(a)
	e.fake.AddLog(b)
	_, err = e.svc.Pull(e.ctx)
	must(t, err)
	var rows []store.StatsHourly
	must(t, e.st.DB.Order("channel_id").Find(&rows).Error)
	if len(rows) != 2 || rows[0].Vendor != "anthropic" || rows[1].Vendor != "" {
		t.Fatalf("rows %+v", rows)
	}
	if rows[1].PricedRequests != 0 || rows[1].RevenueCNY == 0 {
		t.Fatalf("unmanaged channel must have revenue only: %+v", rows[1])
	}
}

func TestPull_ignores_logs_before_backfill_window(t *testing.T) {
	e := newEnv(t)
	must(t, e.st.SetKV(e.ctx, store.KeyStatsBackfillDays, "1"))
	e.fake.AddLog(stdLog(1, time.Now().Unix()-3*86400))
	e.fake.AddLog(stdLog(1, time.Now().Unix()-60))
	_, err := e.svc.Pull(e.ctx)
	must(t, err)
	if ok, _ := e.totals(t); ok != 1 {
		t.Fatalf("success %d", ok)
	}
}

func TestCleanup_switch_off_keeps_newapi_logs(t *testing.T) {
	e := newEnv(t)
	e.fake.AddLog(stdLog(1, time.Now().Unix()-40*86400))
	_, err := e.svc.Pull(e.ctx)
	must(t, err)
	msg, err := e.svc.Cleanup(e.ctx)
	must(t, err)
	if !strings.Contains(msg, "未开启") || e.logCount(t) != 1 {
		t.Fatalf("msg %q logs %d", msg, e.logCount(t))
	}
}

func TestCleanup_never_deletes_past_cursor(t *testing.T) {
	// Given two pulled logs and one newer unpulled log, cleanup on with 0 retention days.
	e := newEnv(t)
	now := time.Now().Unix()
	e.fake.AddLog(stdLog(1, now-7200))
	e.fake.AddLog(stdLog(1, now-3600))
	_, err := e.svc.Pull(e.ctx)
	must(t, err)
	e.fake.AddLog(stdLog(1, now-60))
	must(t, e.st.SetKV(e.ctx, store.KeyNewAPILogCleanup, "true"))
	must(t, e.st.SetKV(e.ctx, store.KeyLogRetentionDays, "0"))

	// When cleaning up.
	_, err = e.svc.Cleanup(e.ctx)
	must(t, err)

	// Then only logs older than the cursor time are gone; the unpulled one survives.
	recent, err := e.svc.Logs.Recent(e.ctx, 1, ports.LogTypeConsume, 0, 10)
	must(t, err)
	if len(recent) != 2 || recent[0].ID != 3 || recent[1].ID != 2 {
		t.Fatalf("kept %+v", recent)
	}
	if ok, _ := e.totals(t); ok != 2 {
		t.Fatalf("aggregates touched: %d", ok)
	}
}

func TestCleanup_keeps_30s_margin_below_cursor(t *testing.T) {
	// Given a pulled log at cursor time T, an older pulled log at T-29, and an unpulled log
	// (higher id) whose created_at is out of order at T-10.
	e := newEnv(t)
	at := time.Now().Unix() - 3600
	e.fake.AddLog(stdLog(1, at-29))
	e.fake.AddLog(stdLog(1, at))
	_, err := e.svc.Pull(e.ctx)
	must(t, err)
	e.fake.AddLog(stdLog(1, at-10))
	e.fake.AddLog(stdLog(1, at-31))
	must(t, e.st.SetKV(e.ctx, store.KeyNewAPILogCleanup, "true"))
	must(t, e.st.SetKV(e.ctx, store.KeyLogRetentionDays, "0"))

	// When cleaning up.
	msg, err := e.svc.Cleanup(e.ctx)
	must(t, err)

	// Then only the row older than cursor.LastAt-30 is deleted.
	if !strings.Contains(msg, "NewAPI 日志 1 条") || e.logCount(t) != 3 {
		t.Fatalf("msg %q logs %d", msg, e.logCount(t))
	}
}

func TestCleanup_drops_old_aggregates(t *testing.T) {
	e := newEnv(t)
	old := time.Now().Unix() - 200*86400
	must(t, e.st.DB.Create(&store.StatsHourly{ChannelID: 1, Model: "m", Hour: old - old%3600, Success: 1}).Error)
	msg, err := e.svc.Cleanup(e.ctx)
	must(t, err)
	if !strings.Contains(msg, "删除聚合 1 行") {
		t.Fatalf("msg %q", msg)
	}
}

func TestRebuild_resets_and_next_pull_refills(t *testing.T) {
	e := newEnv(t)
	at := time.Now().Unix() - 3600
	e.fake.AddLog(stdLog(1, time.Now().Unix()-3*86400))
	for range 3 {
		e.fake.AddLog(stdLog(1, at))
	}
	_, err := e.svc.Pull(e.ctx)
	must(t, err)
	if ok, _ := e.totals(t); ok != 4 {
		t.Fatalf("before %d", ok)
	}
	must(t, e.svc.Rebuild(e.ctx, 1))
	if ok, _ := e.totals(t); ok != 0 {
		t.Fatalf("after reset %d", ok)
	}
	_, err = e.svc.Pull(e.ctx)
	must(t, err)
	if ok, _ := e.totals(t); ok != 3 {
		t.Fatalf("refill %d", ok)
	}
	if _, found, _ := e.st.GetKV(e.ctx, store.KeyStatsRebuildSince); found {
		t.Fatal("rebuild since must be cleared")
	}
	if e.svc.Rebuild(e.ctx, 0) == nil || e.svc.Rebuild(e.ctx, 91) == nil {
		t.Fatal("days out of range must fail")
	}
	st, err := e.svc.Status(e.ctx)
	must(t, err)
	if st.CursorID != 4 || st.Rows != 1 || st.OldestHour == nil || !st.PatchP1 || !st.ErrorLogHint {
		t.Fatalf("status %+v", st)
	}
}

func (e *env) logCount(t *testing.T) int {
	t.Helper()
	recent, err := e.svc.Logs.Recent(e.ctx, 1, ports.LogTypeConsume, 0, 500)
	must(t, err)
	return len(recent)
}
