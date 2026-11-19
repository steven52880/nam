// Package scheduler runs background jobs strictly one at a time (1c2g budget).
package scheduler

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/steven52880/nam/internal/safego"
	"github.com/steven52880/nam/internal/store"
	"gorm.io/gorm"
)

// Job names (stable; used in API and settings).
const (
	JobBalance       = "balance"       // 余额同步
	JobRatio         = "ratio"         // 倍率同步
	JobPriceBook     = "price_book"    // 公开价格库同步
	JobAnnouncements = "announcements" // 公告检查
	JobPullLogs      = "pull_logs"     // 拉取 NewAPI 日志
	JobPricing       = "pricing"       // 定价计算并写入
	JobConsistency   = "consistency"   // 渠道规则检查
	JobCleanup       = "cleanup"       // 清理日志
	JobBackup        = "backup"        // 备份 nam.db
)

// Func is a job body. Returning an error marks the run failed.
type Func func(ctx context.Context) (message string, err error)

// Job describes a registered job.
type Job struct {
	Name    string
	Title   string // 中文名
	Default time.Duration
	Fn      Func
}

// Status is reported to the UI.
type Status struct {
	Name       string `json:"name"`
	Title      string `json:"title"`
	IntervalS  int64  `json:"interval_s"`
	LastStart  int64  `json:"last_start"`
	LastMs     int64  `json:"last_duration_ms"`
	LastResult string `json:"last_result"`
	LastMsg    string `json:"last_message"`
	Running    bool   `json:"running"`
	Queued     bool   `json:"queued"`
	NextAt     int64  `json:"next_at"`
}

// Scheduler is a serial job runner.
type Scheduler struct {
	st   *store.Store
	db   *gorm.DB
	mu   sync.Mutex
	jobs map[string]*entry
	q    chan string
	wg   sync.WaitGroup
	stop context.CancelFunc
}

type entry struct {
	Job
	next    time.Time
	running bool
	queued  bool
	last    store.JobRun
	waiters []chan error
}

// New creates a scheduler.
func New(st *store.Store) *Scheduler {
	return &Scheduler{st: st, db: st.DB, jobs: map[string]*entry{}, q: make(chan string, 64)}
}

// Register adds a job. Call before Start.
func (s *Scheduler) Register(j Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[j.Name] = &entry{Job: j}
}

// Interval returns the configured interval of a job.
func (s *Scheduler) Interval(ctx context.Context, name string) time.Duration {
	s.mu.Lock()
	e, ok := s.jobs[name]
	s.mu.Unlock()
	if !ok {
		return 0
	}
	return s.st.GetDuration(ctx, store.KeyJobIntervalPrefix+name, e.Default)
}

// Start launches the loop; it exits when ctx is cancelled or Stop is called.
func (s *Scheduler) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	s.stop = cancel
	now := time.Now()
	s.mu.Lock()
	for _, e := range s.jobs {
		var last store.JobRun
		if err := s.db.Where("job = ?", e.Name).Order("id desc").Limit(1).Find(&last).Error; err == nil && last.ID != 0 {
			e.last = last
			e.next = time.Unix(last.StartAt, 0).Add(s.st.GetDuration(ctx, store.KeyJobIntervalPrefix+e.Name, e.Default))
		} else {
			// stagger first runs so startup is not a burst
			e.next = now.Add(30 * time.Second)
		}
	}
	s.mu.Unlock()
	safego.GoWG(&s.wg, "scheduler", func() { s.loop(ctx) })
}

// Stop cancels the loop and waits for the current job.
func (s *Scheduler) Stop() {
	if s.stop != nil {
		s.stop()
	}
	s.wg.Wait()
}

// Trigger enqueues a job to run as soon as the runner is free.
// If wait is true it blocks until the job finishes (or ctx ends) and returns its error.
func (s *Scheduler) Trigger(ctx context.Context, name string, wait bool) error {
	s.mu.Lock()
	e, ok := s.jobs[name]
	if !ok {
		s.mu.Unlock()
		return ErrUnknownJob
	}
	var ch chan error
	if wait {
		ch = make(chan error, 1)
		e.waiters = append(e.waiters, ch)
	}
	already := e.queued
	e.queued = true
	s.mu.Unlock()
	if !already {
		select {
		case s.q <- name:
		default:
			return ErrQueueFull
		}
	}
	if !wait {
		return nil
	}
	select {
	case err := <-ch:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Statuses returns all jobs sorted by name.
func (s *Scheduler) Statuses(ctx context.Context) []Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Status, 0, len(s.jobs))
	for _, e := range s.jobs {
		iv := s.st.GetDuration(ctx, store.KeyJobIntervalPrefix+e.Name, e.Default)
		out = append(out, Status{
			Name: e.Name, Title: e.Title, IntervalS: int64(iv / time.Second),
			LastStart: e.last.StartAt, LastMs: e.last.Duration, LastResult: e.last.Result, LastMsg: e.last.Message,
			Running: e.running, Queued: e.queued, NextAt: e.next.Unix(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Scheduler) loop(ctx context.Context) {
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case name := <-s.q:
			s.run(ctx, name)
		case <-tick.C:
			for _, name := range s.due() {
				if ctx.Err() != nil {
					return
				}
				s.run(ctx, name)
			}
		}
	}
}

func (s *Scheduler) due() []string {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	var names []string
	for _, e := range s.jobs {
		if !e.next.IsZero() && now.After(e.next) {
			names = append(names, e.Name)
		}
	}
	sort.Strings(names)
	return names
}

func (s *Scheduler) run(ctx context.Context, name string) {
	s.mu.Lock()
	e, ok := s.jobs[name]
	if !ok {
		s.mu.Unlock()
		return
	}
	e.running = true
	e.queued = false
	waiters := e.waiters
	e.waiters = nil
	s.mu.Unlock()

	start := time.Now()
	var msg string
	err := safego.Run("job:"+name, func() error {
		var err error
		msg, err = e.Fn(ctx)
		return err
	})
	run := store.JobRun{Job: name, StartAt: start.Unix(), Duration: time.Since(start).Milliseconds(), Result: store.ResultOK, Message: msg}
	if err != nil {
		run.Result = store.ResultFail
		run.Message = err.Error()
		slog.Warn("nam: job failed", "job", name, "err", err)
	}
	if dbErr := s.db.Create(&run).Error; dbErr != nil {
		slog.Error("nam: job run record failed", "err", dbErr)
	}

	iv := s.st.GetDuration(ctx, store.KeyJobIntervalPrefix+name, e.Default)
	s.mu.Lock()
	e.running = false
	e.last = run
	e.next = time.Now().Add(iv)
	s.mu.Unlock()
	for _, w := range waiters {
		w <- err
	}
}
