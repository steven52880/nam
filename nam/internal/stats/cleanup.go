package stats

import (
	"context"
	"fmt"
	"time"

	"github.com/steven52880/nam/internal/store"
)

// cleanupMargin keeps raw logs this many seconds older than the cursor time: NewAPI's
// created_at is not strictly in id order, so an unpulled row (id > cursor) may carry a
// created_at slightly below cursor.LastAt.
const cleanupMargin = 30

// Cleanup is the cleanup job body: drop old aggregates and, when enabled (and a cursor
// exists), NewAPI raw logs that are already ingested:
// before = min(now − log_retention_days×86400, cursor.LastAt − 30).
func (s *Service) Cleanup(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Unix()
	var rows int64
	if days := s.St.GetFloat(ctx, store.KeyStatsRetentionDays, 180); days > 0 {
		res := s.St.DB.WithContext(ctx).Where("hour < ?", now-int64(days*86400)).Delete(&store.StatsHourly{})
		if res.Error != nil {
			return "", fmt.Errorf("删除聚合失败：%w", res.Error)
		}
		rows = res.RowsAffected
	}
	if rows > 0 {
		s.InvalidateLatest()
	}
	logPart := "未开启"
	if s.St.GetBool(ctx, store.KeyNewAPILogCleanup, false) {
		cur, ok, err := s.cursor(ctx)
		if err != nil {
			return "", err
		}
		if !ok {
			logPart = "尚未拉取，跳过"
		} else {
			days := max(s.St.GetFloat(ctx, store.KeyLogRetentionDays, 30), 0)
			before := min(now-int64(days*86400), cur.LastAt-cleanupMargin)
			n, err := s.Logs.DeleteBefore(ctx, before)
			if err != nil {
				return fmt.Sprintf("删除聚合 %d 行", rows), fmt.Errorf("删除 NewAPI 日志失败：%w", err)
			}
			logPart = fmt.Sprintf("%d 条", n)
		}
	}
	return fmt.Sprintf("删除聚合 %d 行；NewAPI 日志 %s", rows, logPart), nil
}
