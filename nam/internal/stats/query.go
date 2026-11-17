package stats

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"gorm.io/gorm"
)

// windowSeconds maps a stored-window name to its length.
var windowSeconds = map[string]int64{Window3d: 72 * 3600, Window7d: 168 * 3600}

func hourStart(t int64) int64 { return t - t%3600 }

// validWindow rejects unknown window names.
func validWindow(w string) error {
	if w == WindowLatest {
		return nil
	}
	if _, ok := windowSeconds[w]; ok {
		return nil
	}
	return resp.Invalid("window 必须是 latest、3d 或 7d")
}

// hourRange selects stats_hourly rows with from <= hour < to.
func (s *Service) hourRange(ctx context.Context, from, to int64) *gorm.DB {
	return s.St.DB.WithContext(ctx).Model(&store.StatsHourly{}).Where("hour >= ? AND hour < ?", from, to)
}

// scanRows streams the rows of q into fn one at a time instead of loading the whole slice.
// The single SQLite connection is busy while rows are open: fn must not query the DB.
// (gorm FindInBatches needs a single primary key; stats_hourly's is composite.)
func scanRows(q *gorm.DB, fn func(store.StatsHourly)) error {
	rows, err := q.Rows()
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var r store.StatsHourly
		if err := q.ScanRows(rows, &r); err != nil {
			return err
		}
		fn(r)
	}
	return rows.Err()
}

// windowAggs returns per-channel and per-channel-model aggregates for a window, plus
// the generation time. channelID > 0 restricts stored windows to that channel in SQL.
func (s *Service) windowAggs(ctx context.Context, window string, channelID int) (windowData, int64, error) {
	if err := validWindow(window); err != nil {
		return windowData{}, 0, err
	}
	if window == WindowLatest {
		return s.latestWindow(ctx)
	}
	now := time.Now().Unix()
	q := s.hourRange(ctx, hourStart(now)-windowSeconds[window], hourStart(now)+3600)
	if channelID > 0 {
		q = q.Where("channel_id = ?", channelID)
	}
	wd := newWindowData()
	err := scanRows(q, func(r store.StatsHourly) {
		a := FromRow(r)
		total, ok := wd.ch[r.ChannelID]
		if !ok {
			total = NewAgg()
		}
		total.Merge(a)
		wd.ch[r.ChannelID] = total
		if wd.models[r.ChannelID] == nil {
			wd.models[r.ChannelID] = map[string]Agg{}
		}
		m, ok := wd.models[r.ChannelID][r.Model]
		if !ok {
			m = NewAgg()
		}
		m.Merge(a)
		wd.models[r.ChannelID][r.Model] = m
	})
	if err != nil {
		return windowData{}, 0, err
	}
	return wd, now, nil
}

// ChannelStats is GET /stats/channels.
func (s *Service) ChannelStats(ctx context.Context, window string) (ChannelStatsList, error) {
	out := ChannelStatsList{Window: window, Items: []ChannelStatsItem{}}
	wd, at, err := s.windowAggs(ctx, window, 0)
	if err != nil {
		return out, err
	}
	out.GeneratedAt = at
	chans, err := s.Channels.List(ctx)
	if err != nil {
		return out, resp.Upstream(fmt.Errorf("读取渠道失败：%w", err))
	}
	pr, err := NewPricer(ctx, s.St, s.Backend)
	if err != nil {
		return out, err
	}
	rt, err := s.rate(ctx, pr, chans)
	if err != nil {
		return out, err
	}
	for id, a := range wd.ch {
		m := a.Metrics()
		out.Items = append(out.Items, ChannelStatsItem{
			Metrics: m, ChannelID: id, StdCostPrice: rt.std[id], PriceTier: rt.tierPtr(id),
			QualityTier: qualityTier(m.Requests, m.Wilson),
		})
	}
	sort.Slice(out.Items, func(i, j int) bool { return out.Items[i].ChannelID < out.Items[j].ChannelID })
	out.PatchP1 = s.PatchP1(ctx)
	out.ErrorLogHint, err = s.ErrorLogHint(ctx)
	return out, err
}

// ChannelModels is GET /stats/channels/:id/models.
func (s *Service) ChannelModels(ctx context.Context, id int, window string) (ModelStatsList, error) {
	out := ModelStatsList{Items: []ModelStatsItem{}}
	if err := validWindow(window); err != nil || id <= 0 {
		return out, err
	}
	wd, _, err := s.windowAggs(ctx, window, id)
	if err != nil {
		return out, err
	}
	for model, a := range wd.models[id] {
		out.Items = append(out.Items, ModelStatsItem{Metrics: a.Metrics(), Model: model})
	}
	sort.Slice(out.Items, func(i, j int) bool {
		if out.Items[i].Requests != out.Items[j].Requests {
			return out.Items[i].Requests > out.Items[j].Requests
		}
		return out.Items[i].Model < out.Items[j].Model
	})
	return out, nil
}

// maxTimestamp is the largest accepted unix-second bound (year ~5138).
const maxTimestamp = 1e11

// validRange checks 0 ≤ from < to ≤ 1e11 and the 186-day span cap.
func validRange(from, to int64) error {
	if from < 0 || to > maxTimestamp {
		return resp.Invalid("from/to 超出范围") // checked first so to-from cannot overflow
	}
	if from >= to {
		return resp.Invalid("from 必须小于 to")
	}
	if to-from > MaxSpanSeconds {
		return resp.Invalid("时间跨度不能超过 186 天")
	}
	return nil
}

func validDim(dim string) bool {
	switch dim {
	case DimChannel, DimModel, DimUpstream, DimVendor:
		return true
	}
	return false
}
