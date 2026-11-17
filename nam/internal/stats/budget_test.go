package stats

import (
	"context"
	"testing"
	"time"

	"github.com/steven52880/nam/ports"
)

// blockingLogs is a LogSource whose FetchAfter waits for ctx to end.
type blockingLogs struct{ ports.LogSource }

func (blockingLogs) FetchAfter(ctx context.Context, q ports.LogQuery) ([]ports.LogRecord, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestPull_fetch_timeout_fails_without_commit(t *testing.T) {
	// Given a log source that never answers and a tiny time budget.
	e := newEnv(t)
	e.fake.AddLog(stdLog(1, time.Now().Unix()-60))
	e.svc.Logs = blockingLogs{e.svc.Logs}
	oldMax, oldGrace := pullMaxTime, fetchGrace
	pullMaxTime, fetchGrace = 300*time.Millisecond, 50*time.Millisecond
	t.Cleanup(func() { pullMaxTime, fetchGrace = oldMax, oldGrace })

	// When pulling.
	start := time.Now()
	_, err := e.svc.Pull(e.ctx)

	// Then Pull fails quickly and nothing is committed.
	if err == nil || time.Since(start) > 5*time.Second {
		t.Fatalf("err=%v after %v", err, time.Since(start))
	}
	if _, has, _ := e.svc.cursor(e.ctx); has {
		t.Fatal("cursor must not be written")
	}
	if ok, _ := e.totals(t); ok != 0 {
		t.Fatalf("aggregates written: %d", ok)
	}
}
