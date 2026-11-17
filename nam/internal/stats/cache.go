package stats

import (
	"sync"
	"time"

	"github.com/steven52880/nam/internal/scheduler"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

// Service owns log ingestion, cleanup, rebuild and stats queries.
type Service struct {
	St       *store.Store
	Logs     ports.LogSource
	Channels ports.ChannelStore // audit-wrapped; read only
	Backend  ports.Backend
	Sched    *scheduler.Scheduler // Pull re-queues itself when not caught up; Rebuild triggers a pull
	mu       sync.Mutex           // serialises Pull / Cleanup / Rebuild
	latest   latestCache          // 60 s cache of the "latest" window
}

// latestCache holds the per-channel "latest" results (map[int]latestEntry) and the time
// of the last refresh.
type latestCache struct {
	mu   sync.Mutex
	at   time.Time
	data any
}

// InvalidateLatest makes the next "latest" read refresh. Per-channel results are kept so a
// refresh cut short by its time budget still shows the previous values.
func (s *Service) InvalidateLatest() {
	s.latest.mu.Lock()
	defer s.latest.mu.Unlock()
	s.latest.at = time.Time{}
}
