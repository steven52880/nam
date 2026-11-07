package standalone

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const (
	fakeToken  = "root-token"
	fakeUserID = 1
)

// fakeChannel mirrors the NewAPI model.Channel fields the adapter touches.
type fakeChannel struct {
	ID        int     `json:"id"`
	Type      int     `json:"type"`
	Key       string  `json:"-"`
	Name      string  `json:"name"`
	Status    int     `json:"status"`
	Models    string  `json:"models"`
	Group     string  `json:"group"`
	BaseURL   *string `json:"base_url"`
	Priority  *int64  `json:"priority"`
	Weight    *uint   `json:"weight"`
	Tag       *string `json:"tag"`
	TestModel *string `json:"test_model"`
}

// fakeNewAPI is a faithful-enough NewAPI: envelope replies, create returns no id,
// PUT skips zero non-pointer fields, logs are id-desc paged and require start_timestamp.
type fakeNewAPI struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	channels map[int]*fakeChannel
	nextID   int
	options  map[string]string
	logs     []map[string]any
	logReqs  []map[string][]string // query of every /api/log/ request
	times    []int64               // unix nano receive time per request
}

func newFakeNewAPI(t *testing.T) *fakeNewAPI {
	f := &fakeNewAPI{t: t, channels: map[int]*fakeChannel{}, nextID: 1, options: map[string]string{"GroupRatio": `{"default":1}`}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeNewAPI) config() Config {
	return Config{NewAPIURL: f.srv.URL, Token: fakeToken, UserID: fakeUserID, Secret: "x", AdminPassword: "initial-pass"}
}

func reply(w http.ResponseWriter, ok bool, msg string, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": ok, "message": msg, "data": data})
}

func (f *fakeNewAPI) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.times = append(f.times, timeNow())
	p := r.URL.Path
	if p == "/api/status" {
		reply(w, true, "", map[string]any{"version": "v9.9.9", "quota_per_unit": 500000})
		return
	}
	if r.Header.Get("Authorization") != fakeToken || r.Header.Get("New-Api-User") != strconv.Itoa(fakeUserID) {
		w.WriteHeader(http.StatusUnauthorized)
		reply(w, false, "未登录", nil)
		return
	}
	switch {
	case p == "/api/user/self":
		reply(w, true, "", map[string]any{"id": fakeUserID, "username": "root", "role": 100})
	case strings.HasPrefix(p, "/api/channel"):
		f.serveChannel(w, r)
	case p == "/api/option/":
		f.serveOption(w, r)
	case p == "/api/log/":
		f.serveLog(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeNewAPI) serveOption(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		keys := make([]string, 0, len(f.options))
		for k := range f.options {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		rows := make([]map[string]string, 0, len(keys))
		for _, k := range keys {
			rows = append(rows, map[string]string{"key": k, "value": f.options[k]})
		}
		reply(w, true, "", rows)
		return
	}
	var req struct {
		Key   string `json:"key"`
		Value any    `json:"value"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Key == "" {
		w.WriteHeader(http.StatusBadRequest)
		reply(w, false, "无效的参数", nil)
		return
	}
	s, isStr := req.Value.(string)
	if !isStr {
		b, _ := json.Marshal(req.Value)
		s = string(b)
	}
	f.options[req.Key] = s
	reply(w, true, "", nil)
}
