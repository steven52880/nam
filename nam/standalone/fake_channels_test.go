package standalone

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

func timeNow() int64 { return time.Now().UnixNano() }

func (f *fakeNewAPI) sorted() []*fakeChannel {
	out := make([]*fakeChannel, 0, len(f.channels))
	for _, c := range f.channels {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func paged(r *http.Request, in []*fakeChannel) map[string]any {
	p, _ := strconv.Atoi(r.URL.Query().Get("p"))
	ps, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if p < 1 {
		p = 1
	}
	if ps <= 0 {
		ps = 20
	}
	if ps > 100 {
		ps = 100
	}
	lo, hi := min((p-1)*ps, len(in)), min(p*ps, len(in))
	return map[string]any{"items": in[lo:hi], "collapsed_items": []any{}, "total": len(in)}
}

func (f *fakeNewAPI) serveChannel(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/channel"), "/")
	switch {
	case rest == "" && r.Method == http.MethodGet:
		reply(w, true, "", paged(r, f.sorted()))
	case rest == "search":
		kw := r.URL.Query().Get("keyword")
		var hit []*fakeChannel
		for _, c := range f.sorted() {
			if strings.Contains(c.Name, kw) {
				hit = append(hit, c)
			}
		}
		reply(w, true, "", paged(r, hit))
	case rest == "" && r.Method == http.MethodPost:
		f.createChannel(w, r)
	case rest == "" && r.Method == http.MethodPut:
		f.updateChannel(w, r)
	case strings.HasPrefix(rest, "test/"):
		id, _ := strconv.Atoi(strings.TrimPrefix(rest, "test/"))
		if _, ok := f.channels[id]; !ok {
			reply(w, false, "record not found", nil)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"message":"","time":0.25}`))
	default:
		id, err := strconv.Atoi(rest)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		ch, ok := f.channels[id]
		switch {
		case !ok && r.Method == http.MethodGet:
			reply(w, false, "record not found", nil)
		case r.Method == http.MethodGet:
			reply(w, true, "", ch)
		case r.Method == http.MethodDelete:
			delete(f.channels, id)
			reply(w, true, "", nil)
		default:
			http.NotFound(w, r)
		}
	}
}

type channelBody struct {
	ID        int     `json:"id"`
	Type      int     `json:"type"`
	Key       string  `json:"key"`
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

func (f *fakeNewAPI) createChannel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode    string       `json:"mode"`
		Channel *channelBody `json:"channel"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Channel == nil || req.Mode != "single" {
		reply(w, false, "channel cannot be empty", nil)
		return
	}
	b := req.Channel
	if b.Key == "" {
		reply(w, false, "channel cannot be empty", nil)
		return
	}
	f.channels[f.nextID] = &fakeChannel{
		ID: f.nextID, Type: b.Type, Key: b.Key, Name: b.Name, Status: b.Status, Models: b.Models, Group: b.Group,
		BaseURL: b.BaseURL, Priority: b.Priority, Weight: b.Weight, Tag: b.Tag, TestModel: b.TestModel,
	}
	f.nextID++
	reply(w, true, "", nil) // like NewAPI: no id in the response
}

// updateChannel mimics GORM Updates: zero non-pointer fields are skipped, pointers apply.
func (f *fakeNewAPI) updateChannel(w http.ResponseWriter, r *http.Request) {
	var b channelBody
	if json.NewDecoder(r.Body).Decode(&b) != nil {
		reply(w, false, "bad body", nil)
		return
	}
	c, ok := f.channels[b.ID]
	if !ok {
		reply(w, false, "record not found", nil)
		return
	}
	if b.Type != 0 {
		c.Type = b.Type
	}
	if b.Key != "" {
		c.Key = b.Key
	}
	if b.Name != "" {
		c.Name = b.Name
	}
	if b.Status != 0 {
		c.Status = b.Status
	}
	if b.Models != "" {
		c.Models = b.Models
	}
	if b.Group != "" {
		c.Group = b.Group
	}
	if b.BaseURL != nil {
		c.BaseURL = b.BaseURL
	}
	if b.Priority != nil {
		c.Priority = b.Priority
	}
	if b.Weight != nil {
		c.Weight = b.Weight
	}
	if b.Tag != nil {
		c.Tag = b.Tag
	}
	if b.TestModel != nil {
		c.TestModel = b.TestModel
	}
	reply(w, true, "", c)
}
