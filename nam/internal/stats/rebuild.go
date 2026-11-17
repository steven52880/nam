package stats

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/steven52880/nam/internal/scheduler"
	"github.com/steven52880/nam/internal/store"
	"gorm.io/gorm"
)

// ErrBadDays is returned by Rebuild for days outside 1–90.
var ErrBadDays = errors.New("天数必须在 1–90 之间")

// Rebuild wipes stats_hourly and the cursor, then re-ingests the last `days` days.
func (s *Service) Rebuild(ctx context.Context, days int) error {
	if days < 1 || days > 90 {
		return ErrBadDays
	}
	since := time.Now().Unix() - int64(days)*86400
	if err := s.reset(ctx, since); err != nil {
		return err
	}
	s.latest.mu.Lock()
	s.latest.at, s.latest.data = time.Time{}, nil // full reset, not just a refresh
	s.latest.mu.Unlock()
	if s.Sched == nil {
		return nil
	}
	return s.Sched.Trigger(ctx, scheduler.JobPullLogs, false)
}

func (s *Service) reset(ctx context.Context, since int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.St.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&store.StatsHourly{}).Error; err != nil {
			return err
		}
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&store.LogCursor{}).Error; err != nil {
			return err
		}
		// The re-pull re-observes error logs; a stale value would hide error_log_hint.
		if err := tx.Delete(&store.Setting{}, "key = ?", store.KeyStatsErrorLogSeenAt).Error; err != nil {
			return err
		}
		return setKV(tx, store.KeyStatsRebuildSince, strconv.FormatInt(since, 10))
	})
}

// Status is GET /stats/status.
type Status struct {
	CursorID        int64  `json:"cursor_id"`
	CursorAt        int64  `json:"cursor_at"`
	Rows            int64  `json:"rows"`
	OldestHour      *int64 `json:"oldest_hour"`
	BackfillDays    int    `json:"backfill_days"`
	PatchP1         bool   `json:"patch_p1"`
	ErrorLogHint    bool   `json:"error_log_hint"`
	LastPullAt      int64  `json:"last_pull_at"`
	LastPullMessage string `json:"last_pull_message"`
}

// Status reports ingestion progress.
func (s *Service) Status(ctx context.Context) (Status, error) {
	var out Status
	cur, _, err := s.cursor(ctx)
	if err != nil {
		return out, err
	}
	out.CursorID, out.CursorAt = cur.LastID, cur.LastAt
	db := s.St.DB.WithContext(ctx)
	if err := db.Model(&store.StatsHourly{}).Count(&out.Rows).Error; err != nil {
		return out, err
	}
	if out.Rows > 0 {
		var oldest int64
		if err := db.Model(&store.StatsHourly{}).Select("MIN(hour)").Scan(&oldest).Error; err != nil {
			return out, err
		}
		out.OldestHour = &oldest
	}
	out.BackfillDays = int(s.St.GetFloat(ctx, store.KeyStatsBackfillDays, 7))
	out.PatchP1 = s.PatchP1(ctx)
	if out.ErrorLogHint, err = s.ErrorLogHint(ctx); err != nil {
		return out, err
	}
	var run store.JobRun
	if err := db.Where("job = ?", scheduler.JobPullLogs).Order("id desc").Limit(1).Find(&run).Error; err != nil {
		return out, err
	}
	out.LastPullAt, out.LastPullMessage = run.StartAt, run.Message
	return out, nil
}

// PatchP1 reports whether NewAPI writes use_time_ms (backend flag or seen in a log).
func (s *Service) PatchP1(ctx context.Context) bool {
	if s.St.GetBool(ctx, store.KeyStatsPatchP1Seen, false) {
		return true
	}
	if s.Backend == nil {
		return false
	}
	info, err := s.Backend.Info(ctx)
	return err == nil && info.PatchP1
}

// ErrorLogHint is true when the last 7 days have successes but no error log was ever
// seen in that period (NewAPI's ERROR_LOG_ENABLED is probably off).
func (s *Service) ErrorLogHint(ctx context.Context) (bool, error) {
	from := time.Now().Unix() - 7*86400
	var ok int64
	if err := s.St.DB.WithContext(ctx).Model(&store.StatsHourly{}).Where("hour >= ?", from-from%3600).
		Select("COALESCE(SUM(success), 0)").Scan(&ok).Error; err != nil {
		return false, err
	}
	if ok == 0 {
		return false, nil
	}
	v, found, err := s.St.GetKV(ctx, store.KeyStatsErrorLogSeenAt)
	if err != nil {
		return false, err
	}
	seen, perr := strconv.ParseInt(v, 10, 64)
	return !found || perr != nil || seen < from, nil
}
