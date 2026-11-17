package stats

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/steven52880/nam/internal/scheduler"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Pull limits.
const (
	pullBatch      = 500
	pullMaxBatches = 20
	cursorOverlap  = 600 // seconds subtracted from cursor.LastAt for the time bound
	cursorLogType  = 0   // single merged cursor (consume + error)
)

// Pull time budget (vars so tests can shorten them).
var (
	pullMaxTime = 60 * time.Second
	fetchGrace  = 30 * time.Second // extra time one FetchAfter may take past the deadline
)

type rowKey struct {
	ch    int
	model string
	hour  int64
}

type rowAgg struct {
	group, vendor string
	agg           Agg
}

// pullCounts is the running total of one Pull.
type pullCounts struct {
	total, ok, fail, skip int
}

// batch is one FetchAfter page folded in memory.
type batch struct {
	rows      map[rowKey]*rowAgg
	lastID    int64
	lastAt    int64
	patchSeen bool
	errSeenAt int64
}

// Pull is the pull_logs job body: fetch new consume/error logs and fold them into
// stats_hourly. Each batch and the cursor commit in one transaction, so re-running is
// idempotent.
func (s *Service) Pull(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, hasCur, err := s.cursor(ctx)
	if err != nil {
		return "", err
	}
	since, fromRebuild := s.pullSince(ctx, cur, hasCur)
	pr, err := NewPricer(ctx, s.St, s.Backend)
	if err != nil {
		return "", err
	}
	deadline := time.Now().Add(pullMaxTime)
	var n pullCounts
	caughtUp := false
	for i := 0; i < pullMaxBatches && time.Now().Before(deadline); i++ {
		// Each fetch gets the remaining budget plus a grace period; on timeout the batch
		// is not committed and Pull fails.
		fctx, cancel := context.WithTimeout(ctx, max(time.Until(deadline), 0)+fetchGrace)
		recs, err := s.Logs.FetchAfter(fctx, ports.LogQuery{
			AfterID: cur.LastID, Since: since, Types: []int{ports.LogTypeConsume, ports.LogTypeError}, Limit: pullBatch,
		})
		cancel()
		if err != nil {
			return pullMessage(n, cur.LastID, false), fmt.Errorf("拉取日志失败：%w", err)
		}
		if len(recs) == 0 {
			caughtUp = true
			break
		}
		b := fold(recs, pr, &n)
		if err := s.commit(ctx, b, fromRebuild); err != nil {
			return pullMessage(n, cur.LastID, false), fmt.Errorf("写入统计失败：%w", err)
		}
		fromRebuild = false
		cur.LastID, cur.LastAt = b.lastID, b.lastAt
		if len(recs) < pullBatch {
			caughtUp = true
			break
		}
	}
	if n.total > 0 {
		s.InvalidateLatest()
	}
	if !caughtUp && s.Sched != nil {
		if err := s.Sched.Trigger(ctx, scheduler.JobPullLogs, false); err != nil {
			return pullMessage(n, cur.LastID, false), fmt.Errorf("重新排队失败：%w", err)
		}
	}
	return pullMessage(n, cur.LastID, caughtUp), nil
}

func pullMessage(n pullCounts, id int64, caughtUp bool) string {
	msg := fmt.Sprintf("处理 %d 条（成功 %d、失败 %d、跳过 %d），游标 #%d", n.total, n.ok, n.fail, n.skip, id)
	if !caughtUp {
		msg = "已" + msg + "，未追平"
	}
	return msg
}

// pullSince picks the time bound. fromRebuild reports that stats_rebuild_since was used
// and must be cleared with the first committed batch.
func (s *Service) pullSince(ctx context.Context, cur store.LogCursor, hasCur bool) (int64, bool) {
	if hasCur {
		return max(cur.LastAt-cursorOverlap, 1), false
	}
	if v, ok, err := s.St.GetKV(ctx, store.KeyStatsRebuildSince); err == nil && ok {
		if ts, err := strconv.ParseInt(v, 10, 64); err == nil && ts > 0 {
			return ts, true
		}
	}
	days := s.St.GetFloat(ctx, store.KeyStatsBackfillDays, 7)
	if days < 1 || days > 90 {
		days = 7
	}
	return time.Now().Unix() - int64(days*86400), false
}

func (s *Service) cursor(ctx context.Context) (store.LogCursor, bool, error) {
	var c store.LogCursor
	err := s.St.DB.WithContext(ctx).First(&c, "log_type = ?", cursorLogType).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return store.LogCursor{LogType: cursorLogType}, false, nil
	}
	return c, err == nil, err
}

// fold parses one page into per-key aggregates. The cursor advances over skipped logs.
func fold(recs []ports.LogRecord, pr *Pricer, n *pullCounts) batch {
	b := batch{rows: map[rowKey]*rowAgg{}}
	for _, r := range recs {
		if r.ID > b.lastID {
			b.lastID, b.lastAt = r.ID, r.CreatedAt
		}
		n.total++
		smp, ok := Parse(r)
		if !ok {
			n.skip++
			continue
		}
		if smp.OK {
			n.ok++
			b.patchSeen = b.patchSeen || smp.UseTimeMS
		} else {
			n.fail++
			b.errSeenAt = max(b.errSeenAt, smp.CreatedAt)
		}
		k := rowKey{ch: smp.ChannelID, model: smp.Model, hour: smp.Hour}
		ra, ok := b.rows[k]
		if !ok {
			ra = &rowAgg{group: smp.Group, vendor: pr.VendorOf(smp.Group, smp.ChannelID), agg: NewAgg()}
			b.rows[k] = ra
		}
		ra.agg.Add(smp, pr.Price(smp))
	}
	return b
}

// commit merges a batch into stats_hourly and moves the cursor in one transaction.
func (s *Service) commit(ctx context.Context, b batch, clearRebuild bool) error {
	return s.St.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for k, ra := range b.rows {
			var row store.StatsHourly
			err := tx.First(&row, "channel_id = ? AND model = ? AND hour = ?", k.ch, k.model, k.hour).Error
			switch {
			case errors.Is(err, gorm.ErrRecordNotFound):
				row = store.StatsHourly{ChannelID: k.ch, Model: k.model, Hour: k.hour, Group: ra.group, Vendor: ra.vendor}
			case err != nil:
				return err
			}
			a := FromRow(row)
			a.Merge(ra.agg)
			a.ApplyTo(&row)
			// Explicit upsert: gorm Save treats zero-valued key parts (model "", log_type 0) as new rows.
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error; err != nil {
				return err
			}
		}
		cur := store.LogCursor{LogType: cursorLogType, LastID: b.lastID, LastAt: b.lastAt}
		if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&cur).Error; err != nil {
			return err
		}
		if clearRebuild {
			if err := tx.Delete(&store.Setting{}, "key = ?", store.KeyStatsRebuildSince).Error; err != nil {
				return err
			}
		}
		if b.patchSeen {
			if err := setKV(tx, store.KeyStatsPatchP1Seen, "true"); err != nil {
				return err
			}
		}
		if b.errSeenAt > 0 {
			return bumpKV(tx, store.KeyStatsErrorLogSeenAt, b.errSeenAt)
		}
		return nil
	})
}

func setKV(tx *gorm.DB, key, value string) error {
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&store.Setting{Key: key, Value: value}).Error
}

// bumpKV stores v only when it is larger than the stored integer.
func bumpKV(tx *gorm.DB, key string, v int64) error {
	var st store.Setting
	err := tx.First(&st, "key = ?", key).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if old, perr := strconv.ParseInt(st.Value, 10, 64); err == nil && perr == nil && old >= v {
		return nil
	}
	return setKV(tx, key, strconv.FormatInt(v, 10))
}
