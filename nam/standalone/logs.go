package standalone

import (
	"context"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/steven52880/nam/ports"
)

const (
	logPageSize  = 100 // NewAPI caps page_size at 100
	logMaxLimit  = 500
	logDefaultTo = 24 * time.Hour
)

// logStore implements ports.LogSource over GET/DELETE /api/log/. It never calls
// /api/log/stat or /api/log/performance (both are heavy aggregate queries).
type logStore struct{ c *Client }

type logPage struct {
	Items []ports.LogRecord `json:"items"`
	Total int               `json:"total"`
}

func since(ts int64) int64 {
	if ts > 0 {
		return ts
	}
	return time.Now().Add(-logDefaultTo).Unix()
}

func (s logStore) page(ctx context.Context, q url.Values, p int) (logPage, error) {
	q.Set("p", strconv.Itoa(p))
	q.Set("page_size", strconv.Itoa(logPageSize))
	var res logPage
	err := s.c.call(ctx, http.MethodGet, "/api/log/", q, nil, &res)
	return res, err
}

// FetchAfter pages newest-first (NewAPI orders by id desc) until it reaches AfterID, then
// returns the collected rows ascending, capped at Limit (oldest first, so the caller can
// advance its cursor without gaps). With several Types it runs one series per type.
func (s logStore) FetchAfter(ctx context.Context, q ports.LogQuery) ([]ports.LogRecord, error) {
	limit := q.Limit
	if limit <= 0 || limit > logMaxLimit {
		limit = logMaxLimit
	}
	types := q.Types
	if len(types) == 0 {
		types = []int{0} // 0 = every type
	}
	start := since(q.Since)
	seen := map[int64]struct{}{}
	var all []ports.LogRecord
	for _, t := range types {
		for p := 1; p <= maxPages; p++ {
			params := url.Values{"type": {strconv.Itoa(t)}, "start_timestamp": {strconv.FormatInt(start, 10)}}
			res, err := s.page(ctx, params, p)
			if err != nil {
				return nil, err
			}
			reached := false
			for _, r := range res.Items {
				if r.ID <= q.AfterID {
					reached = true
					continue
				}
				if _, dup := seen[r.ID]; !dup {
					seen[r.ID] = struct{}{}
					all = append(all, r)
				}
			}
			if reached || len(res.Items) < logPageSize {
				break
			}
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// Recent returns the newest logs of one channel (NewAPI supports the `channel` filter).
func (s logStore) Recent(ctx context.Context, channelID, logType int, sinceTS int64, limit int) ([]ports.LogRecord, error) {
	if limit <= 0 || limit > logMaxLimit {
		limit = logMaxLimit
	}
	out := make([]ports.LogRecord, 0, limit)
	for p := 1; p <= maxPages && len(out) < limit; p++ {
		params := url.Values{
			"type":            {strconv.Itoa(logType)},
			"channel":         {strconv.Itoa(channelID)},
			"start_timestamp": {strconv.FormatInt(since(sinceTS), 10)},
		}
		res, err := s.page(ctx, params, p)
		if err != nil {
			return nil, err
		}
		out = append(out, res.Items...)
		if len(res.Items) < logPageSize {
			break
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// DeleteBefore calls DELETE /api/log/?target_timestamp=; NewAPI returns the deleted count.
func (s logStore) DeleteBefore(ctx context.Context, before int64) (int64, error) {
	var n int64
	q := url.Values{"target_timestamp": {strconv.FormatInt(before, 10)}}
	err := s.c.call(ctx, http.MethodDelete, "/api/log/", q, nil, &n)
	return n, err
}
