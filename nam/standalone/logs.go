package standalone

import (
	"context"
	"errors"
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

// logMaxPages bounds one FetchAfter scan (a var so tests can lower it).
var logMaxPages = maxPages

// errScanTooLong is returned instead of a partial result when the page cap is hit.
var errScanTooLong = errors.New("日志分页超过上限，未读完")

// FetchAfter returns rows of the requested types with id > AfterID ascending, capped at
// Limit, so the caller can advance its cursor without gaps.
//
// ONE scan over every type (type=0) is filtered client-side: scanning each type separately
// could skip a row of type A inserted after A's scan with an id below a type-B row read
// later. NewAPI pages are id desc, so the scan runs from the OLDEST page towards page 1 and
// stops once Limit requested-type rows are collected; then only rows with id ≤ the last
// collected id are returned, so every id below the cut has been seen.
//
// Positions are tracked from the OLDEST row (stable while new rows are appended): `done`
// rows from the old end are consumed. Each page number is derived from the latest total;
// if inserts between requests moved the page past `done`, the step is retried with the new
// total, so reading backwards can repeat rows (deduped by id) but not miss one.
//
// Residual risk (standalone mode only; embedded mode reads SQL directly): NewAPI's
// GET /api/log/ runs COUNT and SELECT as separate queries. Rows inserted between them
// inside ONE request shift the page without changing the reported total, so a few rows at
// a page boundary can be missed.
func (s logStore) FetchAfter(ctx context.Context, q ports.LogQuery) ([]ports.LogRecord, error) {
	limit := q.Limit
	if limit <= 0 || limit > logMaxLimit {
		limit = logMaxLimit
	}
	want, every := map[int]bool{}, len(q.Types) == 0
	for _, t := range q.Types {
		want[t] = true
		every = every || t == 0 // 0 = every type
	}
	match := func(t int) bool { return every || want[t] }
	params := func() url.Values {
		return url.Values{"type": {"0"}, "start_timestamp": {strconv.FormatInt(since(q.Since), 10)}}
	}
	first, err := s.page(ctx, params(), 1)
	if err != nil {
		return nil, err
	}
	total, done, hits := first.Total, 0, 0
	seen := map[int64]struct{}{}
	var rows []ports.LogRecord // every type, id > AfterID
	for step := 0; done < total && hits < limit; step++ {
		if step >= logMaxPages {
			return nil, errScanTooLong
		}
		p := (total-done-1)/logPageSize + 1
		res, err := s.page(ctx, params(), p)
		if err != nil {
			return nil, err
		}
		if lo := res.Total - p*logPageSize; lo > done {
			total = res.Total // rows were appended meanwhile; recompute the page
			continue
		}
		for _, r := range res.Items {
			if _, dup := seen[r.ID]; !dup && r.ID > q.AfterID {
				seen[r.ID] = struct{}{}
				rows = append(rows, r)
				if match(r.Type) {
					hits++
				}
			}
		}
		total, done = res.Total, res.Total-(p-1)*logPageSize
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	out := make([]ports.LogRecord, 0, min(hits, limit))
	for _, r := range rows {
		if match(r.Type) {
			out = append(out, r)
			if len(out) == limit {
				break // rows above this id may be unread
			}
		}
	}
	return out, nil
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
