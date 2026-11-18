package stats

import (
	"context"
	"sort"
	"strconv"
	"time"

	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"gorm.io/gorm"
)

// Series is GET /stats/series. dim == "" means the whole site; otherwise key filters on
// that dimension in SQL.
func (s *Service) Series(ctx context.Context, from, to int64, dim, key, bucket string) (SeriesResult, error) {
	out := SeriesResult{Bucket: bucket, Points: []SeriesPoint{}}
	if err := validRange(from, to); err != nil {
		return out, err
	}
	if bucket != BucketHour && bucket != BucketDay {
		return out, resp.Invalid("bucket 必须是 hour 或 day")
	}
	if dim != "" && !validDim(dim) {
		return out, resp.Invalid("dim 必须是 channel、model、upstream 或 vendor")
	}
	q, ok, err := s.seriesQuery(ctx, from, to, dim, key)
	if err != nil || !ok {
		return out, err
	}
	buckets := map[int64]Agg{}
	err = scanRows(q, func(r store.StatsHourly) {
		t := bucketStart(r.Hour, bucket)
		a, ok := buckets[t]
		if !ok {
			a = NewAgg()
		}
		a.Merge(FromRow(r))
		buckets[t] = a
	})
	if err != nil {
		return out, err
	}
	usd := s.St.USDRate(ctx)
	for t, a := range buckets {
		out.Points = append(out.Points, SeriesPoint{Metrics: a.Metrics(usd), T: t})
	}
	sort.Slice(out.Points, func(i, j int) bool { return out.Points[i].T < out.Points[j].T })
	return out, nil
}

// seriesQuery builds the filtered row query; ok == false means no row can match.
func (s *Service) seriesQuery(ctx context.Context, from, to int64, dim, key string) (*gorm.DB, bool, error) {
	q := s.hourRange(ctx, from, to)
	switch dim {
	case "":
		return q, true, nil
	case DimChannel:
		id, err := strconv.Atoi(key)
		return q.Where("channel_id = ?", id), err == nil, nil
	case DimModel:
		return q.Where("model = ?", key), true, nil
	case DimVendor:
		return q.Where("vendor = ?", key), true, nil
	}
	ids, err := s.upstreamChannels(ctx, from, to, key)
	if err != nil || len(ids) == 0 {
		return nil, false, err
	}
	return q.Where("channel_id IN ?", ids), true, nil
}

// upstreamChannels lists the channel ids belonging to upstream key; key "" means channels
// without channel_ext or with a null upstream_id (taken from the stats rows in range).
func (s *Service) upstreamChannels(ctx context.Context, from, to int64, key string) ([]int, error) {
	db := s.St.DB.WithContext(ctx)
	if key != "" {
		up, err := strconv.ParseUint(key, 10, 64)
		if err != nil {
			return nil, nil
		}
		var ids []int
		err = db.Model(&store.ChannelExt{}).Where("upstream_id = ?", up).Pluck("channel_id", &ids).Error
		return ids, err
	}
	var exts []store.ChannelExt
	if err := db.Select("channel_id", "upstream_id").Find(&exts).Error; err != nil {
		return nil, err
	}
	managed := map[int]bool{}
	for _, e := range exts {
		if e.UpstreamID != nil {
			managed[e.ChannelID] = true
		}
	}
	var seen []int
	if err := s.hourRange(ctx, from, to).Distinct().Pluck("channel_id", &seen).Error; err != nil {
		return nil, err
	}
	var ids []int
	for _, id := range seen {
		if !managed[id] {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// bucketStart is the stored hour, or the local-midnight of its natural day.
func bucketStart(hour int64, bucket string) int64 {
	if bucket == BucketHour {
		return hour
	}
	t := time.Unix(hour, 0).In(time.Local)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local).Unix()
}
