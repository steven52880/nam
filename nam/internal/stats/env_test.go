package stats

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
	"github.com/steven52880/nam/ports/portstest"
)

func f64(v float64) *float64 { return &v }

type env struct {
	ctx  context.Context
	st   *store.Store
	fake *portstest.Fake
	svc  *Service
}

// newEnv opens a temp nam.db with: upstream (rate 0.5), vendor anthropic (group
// "anthropic"), channel 1 managed (manual discount 0.35), price for
// test-model 3 / 15 / cache read 0.3.
func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "nam.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	if err := st.EnsureDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	up := store.Upstream{Name: "up", Type: store.UpstreamNewAPI, BaseURL: "http://x", CurrencyRate: 0.5}
	must(t, st.DB.Create(&up).Error)
	must(t, st.DB.Create(&store.Vendor{Name: "anthropic", GroupName: "anthropic", Order: 1}).Error)
	must(t, st.DB.Create(&store.ChannelExt{ChannelID: 1, UpstreamID: &up.ID, ManualDiscount: f64(0.35)}).Error)
	must(t, st.DB.Create(&store.OfficialPrice{Model: "test-model", Input: f64(3), Output: f64(15), CacheRead: f64(0.3)}).Error)
	fake := portstest.NewFake()
	p := fake.Ports()
	return &env{ctx: ctx, st: st, fake: fake, svc: &Service{St: st, Logs: p.Logs, Channels: p.Channels, Backend: p.Backend}}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// stdLog is the plan's standard request on channel ch.
func stdLog(ch int, at int64) ports.LogRecord {
	return ports.LogRecord{
		CreatedAt: at, Type: ports.LogTypeConsume, ModelName: "test-model", ChannelID: ch, Group: "anthropic",
		PromptTokens: 1000, CompletionTokens: 500, IsStream: true, Quota: 1743,
		Other: `{"cache_tokens":200,"frt":300,"use_time_ms":2300}`,
	}
}

func errLog(ch int, at int64) ports.LogRecord {
	return ports.LogRecord{CreatedAt: at, Type: ports.LogTypeError, ModelName: "test-model", ChannelID: ch,
		Other: `{"status_code":500}`}
}

func (e *env) totals(t *testing.T) (success, fail int64) {
	t.Helper()
	var rows []store.StatsHourly
	must(t, e.st.DB.Find(&rows).Error)
	for _, r := range rows {
		success += r.Success
		fail += r.Fail
	}
	return success, fail
}

func (e *env) cursor(t *testing.T) store.LogCursor {
	t.Helper()
	c, _, err := e.svc.cursor(e.ctx)
	must(t, err)
	return c
}
