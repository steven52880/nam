package stats

import (
	"context"
	"sort"
	"time"

	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

const (
	latestTTL    = 60 * time.Second
	latestLimit  = 100
	latestWindow = 86400 // seconds
)

// latestTimeout is the budget of one refresh (a var so tests can shorten it).
var latestTimeout = 20 * time.Second

// windowData is the per-channel and per-channel-model aggregates of one window.
type windowData struct {
	ch     map[int]Agg
	models map[int]map[string]Agg
}

func newWindowData() windowData {
	return windowData{ch: map[int]Agg{}, models: map[int]map[string]Agg{}}
}

// latestEntry is one channel's "latest" result and when it was computed.
type latestEntry struct {
	at     time.Time
	has    bool // false: the channel had no usable samples
	total  Agg
	models map[string]Agg
}

// latestWindow returns the "latest" window (each channel's last 100 logs). A refresh runs
// at most every 60 s; the cache mutex is held while refreshing so concurrent callers
// compute once.
func (s *Service) latestWindow(ctx context.Context) (windowData, int64, error) {
	s.latest.mu.Lock()
	defer s.latest.mu.Unlock()
	entries, _ := s.latest.data.(map[int]latestEntry)
	if entries == nil || time.Since(s.latest.at) >= latestTTL {
		next, err := s.refreshLatest(ctx, entries)
		if err != nil {
			return windowData{}, 0, err
		}
		entries = next
		s.latest.at, s.latest.data = time.Now(), next
	}
	wd := newWindowData()
	for id, e := range entries {
		if e.has {
			wd.ch[id], wd.models[id] = e.total, e.models
		}
	}
	return wd, s.latest.at.Unix(), nil
}

// refreshLatest recomputes candidates within the time budget, never-computed channels
// first, then oldest-computed first. Channels not reached keep their previous result, so
// a slow backend cannot starve the channels at the end of the list.
func (s *Service) refreshLatest(ctx context.Context, prev map[int]latestEntry) (map[int]latestEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, latestTimeout)
	defer cancel()
	candidates, err := s.latestCandidates(ctx)
	if err != nil {
		return nil, err
	}
	next := make(map[int]latestEntry, len(candidates))
	for _, id := range candidates {
		if e, ok := prev[id]; ok {
			next[id] = e // dropped channels fall out here
		}
	}
	if len(candidates) == 0 {
		return next, nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, okA := next[candidates[i]]
		b, okB := next[candidates[j]]
		if okA != okB {
			return !okA // never computed first
		}
		return a.at.Before(b.at)
	})
	pr, err := NewPricer(ctx, s.St, s.Backend)
	if err != nil {
		return nil, err
	}
	since := time.Now().Unix() - latestWindow
	for _, id := range candidates {
		if ctx.Err() != nil {
			break // out of time: the rest keep their previous result
		}
		samples, err := s.recentSamples(ctx, id, since)
		if ctx.Err() != nil {
			break
		}
		e := next[id]
		e.at = time.Now()
		if err == nil { // on a backend error keep the old aggregates but move on
			e = latestEntry{at: e.at, has: len(samples) > 0, total: NewAgg(), models: map[string]Agg{}}
			for _, smp := range samples {
				priced := pr.Price(smp)
				e.total.Add(smp, priced)
				m, ok := e.models[smp.Model]
				if !ok {
					m = NewAgg()
				}
				m.Add(smp, priced)
				e.models[smp.Model] = m
			}
		}
		next[id] = e
	}
	return next, nil
}

// latestCandidates are the NewAPI channels that have stats_hourly rows in the last 24 h.
func (s *Service) latestCandidates(ctx context.Context) ([]int, error) {
	chans, err := s.Channels.List(ctx)
	if err != nil {
		return nil, err
	}
	from := time.Now().Unix() - latestWindow
	from -= from % 3600
	var active []int
	if err := s.St.DB.WithContext(ctx).Model(&store.StatsHourly{}).Where("hour >= ?", from).
		Distinct().Pluck("channel_id", &active).Error; err != nil {
		return nil, err
	}
	has := make(map[int]bool, len(active))
	for _, id := range active {
		has[id] = true
	}
	var out []int
	for _, ch := range chans {
		if has[ch.ID] {
			out = append(out, ch.ID)
		}
	}
	return out, nil
}

// recentSamples merges the newest consume and error logs of a channel, drops test logs
// and keeps the newest 100 parsed samples.
func (s *Service) recentSamples(ctx context.Context, id int, since int64) ([]Sample, error) {
	var recs []ports.LogRecord
	for _, typ := range []int{ports.LogTypeConsume, ports.LogTypeError} {
		part, err := s.Logs.Recent(ctx, id, typ, since, latestLimit)
		if err != nil {
			return nil, err
		}
		recs = append(recs, part...)
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].ID > recs[j].ID })
	var out []Sample
	for _, r := range recs {
		if smp, ok := Parse(r); ok {
			out = append(out, smp)
			if len(out) == latestLimit {
				break
			}
		}
	}
	return out, nil
}
