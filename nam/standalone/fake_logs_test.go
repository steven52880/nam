package standalone

import (
	"net/http"
	"sort"
	"strconv"
)

// seedLogs stores n consume logs with ids 1..n (created_at = 1000+id).
func (f *fakeNewAPI) seedLogs(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := 1; i <= n; i++ {
		f.logs = append(f.logs, map[string]any{
			"id": i, "created_at": 1000 + i, "type": 2, "channel": 7, "model_name": "m", "content": "",
		})
	}
}

// seedTypedLogs stores n logs with ids 1..n whose type is typeOf(id).
func (f *fakeNewAPI) seedTypedLogs(n int, typeOf func(id int) int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := 1; i <= n; i++ {
		f.logs = append(f.logs, map[string]any{
			"id": i, "created_at": 1000 + i, "type": typeOf(i), "channel": 7, "model_name": "m", "content": "",
		})
	}
}

// appendLogLocked adds one log with the next id; the caller holds f.mu.
func (f *fakeNewAPI) appendLogLocked(typ int) {
	id := len(f.logs) + 1
	f.logs = append(f.logs, map[string]any{
		"id": id, "created_at": 1000 + id, "type": typ, "channel": 7, "model_name": "m", "content": "",
	})
}

func (f *fakeNewAPI) logRequests() []map[string][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]map[string][]string(nil), f.logReqs...)
}

func (f *fakeNewAPI) serveLog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if r.Method == http.MethodDelete {
		before, _ := strconv.ParseInt(q.Get("target_timestamp"), 10, 64)
		if before == 0 {
			reply(w, false, "target timestamp is required", nil)
			return
		}
		var keep []map[string]any
		for _, l := range f.logs {
			if l["created_at"].(int) >= int(before) {
				keep = append(keep, l)
			}
		}
		deleted := len(f.logs) - len(keep)
		f.logs = keep
		reply(w, true, "", deleted)
		return
	}
	f.logReqs = append(f.logReqs, q)
	if f.afterLogGet != nil {
		defer f.afterLogGet(len(f.logReqs)) // runs after the reply, still under f.mu
	}
	if q.Get("start_timestamp") == "" {
		w.WriteHeader(http.StatusBadRequest)
		reply(w, false, "start_timestamp is required", nil)
		return
	}
	start, _ := strconv.Atoi(q.Get("start_timestamp"))
	typ, _ := strconv.Atoi(q.Get("type"))
	ch, _ := strconv.Atoi(q.Get("channel"))
	p, _ := strconv.Atoi(q.Get("p"))
	ps, _ := strconv.Atoi(q.Get("page_size"))
	if p < 1 {
		p = 1
	}
	if ps <= 0 {
		ps = 10
	}
	if ps > 100 {
		ps = 100
	}
	var hit []map[string]any
	for _, l := range f.logs {
		switch {
		case l["created_at"].(int) < start:
		case typ != 0 && l["type"].(int) != typ:
		case ch != 0 && l["channel"].(int) != ch:
		default:
			hit = append(hit, l)
		}
	}
	sort.Slice(hit, func(i, j int) bool { return hit[i]["id"].(int) > hit[j]["id"].(int) })
	lo, hi := min((p-1)*ps, len(hit)), min(p*ps, len(hit))
	reply(w, true, "", map[string]any{"page": p, "page_size": ps, "total": len(hit), "items": hit[lo:hi]})
}
