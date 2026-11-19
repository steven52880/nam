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

func hourStart(t int64) int64 { return t - t%3600 }

const errWindow = "window 必须是 latest，或 1h–4464h / 1d–186d 这样的时长"

// windowSpan parses "<N>h" or "<N>d" (N a positive integer without leading zeros,
// total length ≤ MaxSpanSeconds) into seconds. "latest" and anything else is an error.
func windowSpan(w string) (int64, error) {
	n := len(w)
	if n < 2 || (w[n-1] != 'h' && w[n-1] != 'd') || w[0] < '1' || w[0] > '9' || n > 6 {
		return 0, resp.Invalid(errWindow)
	}
	var num int64
	for _, c := range w[:n-1] {
		if c < '0' || c > '9' {
			return 0, resp.Invalid(errWindow)
		}
		num = num*10 + int64(c-'0')
	}
	unit := int64(3600)
	if w[n-1] == 'd' {
		unit = 86400
	}
	if num*unit > MaxSpanSeconds {
		return 0, resp.Invalid(errWindow)
	}
	return num * unit, nil
}

// validWindow accepts "latest" or a duration accepted by windowSpan.
func validWindow(w string) error {
	if w == WindowLatest {
		return nil
	}
	_, err := windowSpan(w)
	return err
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
	if window == WindowLatest {
		return s.latestWindow(ctx)
	}
	span, err := windowSpan(window)
	if err != nil {
		return windowData{}, 0, err
	}
	now := time.Now().Unix()
	q := s.hourRange(ctx, hourStart(now)-span, hourStart(now)+3600)
	if channelID > 0 {
		q = q.Where("channel_id = ?", channelID)
	}
	wd := newWindowData()
	err = scanRows(q, func(r store.StatsHourly) {
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

// ChannelStats is GET /stats/channels. A non-empty model narrows every channel's numbers
// to that model (channels without it are skipped); empty keeps the channel totals.
func (s *Service) ChannelStats(ctx context.Context, window, model string) (ChannelStatsList, error) {
	out := ChannelStatsList{Window: window, Model: model, Items: []ChannelStatsItem{}}
	wd, at, err := s.windowAggs(ctx, window, 0)
	if err != nil {
		return out, err
	}
	out.GeneratedAt = at
	chans, err := s.Channels.List(ctx)
	if err != nil {
		return out, resp.Upstream(fmt.Errorf("读取渠道失败：%w", err))
	}
	pr, err := NewPricer(ctx, s.St, s.Backend, s.Channels)
	if err != nil {
		return out, err
	}
	rt, err := s.rate(ctx, pr, chans, model)
	if err != nil {
		return out, err
	}
	for id, a := range wd.ch {
		if model != "" {
			ma, ok := wd.models[id][model]
			if !ok {
				continue
			}
			a = ma
		}
		m := a.Metrics(pr.usdRate)
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
	usd := s.St.USDRate(ctx)
	for model, a := range wd.models[id] {
		out.Items = append(out.Items, ModelStatsItem{Metrics: a.Metrics(usd), Model: model})
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
