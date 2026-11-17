package stats

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

// slowLogs delays every Recent call and records which channels were queried.
type slowLogs struct {
	ports.LogSource
	delay time.Duration
	mu    sync.Mutex
	seen  map[int]bool
}

func (l *slowLogs) Recent(ctx context.Context, ch, typ int, since int64, limit int) ([]ports.LogRecord, error) {
	select {
	case <-time.After(l.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	l.mu.Lock()
	l.seen[ch] = true
	l.mu.Unlock()
	return l.LogSource.Recent(ctx, ch, typ, since, limit)
}

func (l *slowLogs) take() map[int]bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := l.seen
	l.seen = map[int]bool{}
	return out
}

func TestLatest_slow_backend_refreshes_oldest_channels_next(t *testing.T) {
	// Given 4 active channels and a backend slow enough that one refresh reaches ~2 of them.
	e := newEnv(t)
	h := hourStart(time.Now().Unix())
	for i := range 4 {
		id, err := e.fake.Create(e.ctx, ports.ChannelCreate{Name: string(rune('a' + i)), Models: []string{"test-model"}})
		must(t, err)
		e.fake.AddLog(stdLog(id, time.Now().Unix()-60))
		must(t, e.st.DB.Create(&store.StatsHourly{ChannelID: id, Model: "test-model", Hour: h, Success: 1}).Error)
	}
	slow := &slowLogs{LogSource: e.svc.Logs, delay: 40 * time.Millisecond, seen: map[int]bool{}}
	e.svc.Logs = slow
	old := latestTimeout
	latestTimeout = 200 * time.Millisecond
	t.Cleanup(func() { latestTimeout = old })

	// When refreshing twice (the TTL is bypassed by invalidating in between).
	wd1, _, err := e.svc.latestWindow(e.ctx)
	must(t, err)
	first := slow.take()
	e.svc.InvalidateLatest()
	wd2, _, err := e.svc.latestWindow(e.ctx)
	must(t, err)
	second := slow.take()

	// Then the second refresh covers channels the first did not reach, and results the
	// first refresh produced are kept.
	if len(first) == 0 || len(first) == 4 {
		t.Fatalf("first refresh should cover some but not all channels: %v", first)
	}
	fresh := false
	for id := range second {
		if !first[id] {
			fresh = true
		}
	}
	if !fresh {
		t.Fatalf("second refresh repeated the same channels: first %v second %v", first, second)
	}
	if len(wd2.ch) < len(wd1.ch) {
		t.Fatalf("results lost: %d then %d channels", len(wd1.ch), len(wd2.ch))
	}

	// And the 60 s TTL still gates refreshing.
	if _, _, err := e.svc.latestWindow(e.ctx); err != nil || len(slow.take()) != 0 {
		t.Fatalf("cached read must not query the backend: %v", err)
	}
}
