// Package portstest provides an in-memory fake of every port and a contract test
// suite that both real adapters (standalone, nambridge) must pass.
package portstest

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/ports"
)

// Fake implements all ports in memory. Role is taken from header "X-Fake-Role".
type Fake struct {
	mu       sync.Mutex
	nextID   int
	channels map[int]ports.Channel
	options  map[string]string
	logs     []ports.LogRecord
	Writes   int // number of successful Set/Create/Update/Delete calls
}

// NewFake returns an empty fake.
func NewFake() *Fake {
	return &Fake{nextID: 1, channels: map[int]ports.Channel{}, options: map[string]string{}}
}

// Ports returns the fake wired as ports.Ports.
func (f *Fake) Ports() ports.Ports {
	return ports.Ports{Channels: f, Options: (*fakeOptions)(f), Logs: (*fakeLogs)(f), Auth: fakeAuth{}, Backend: f}
}

// AddLog appends a log record (ID assigned if zero).
func (f *Fake) AddLog(r ports.LogRecord) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.ID == 0 {
		r.ID = int64(len(f.logs) + 1)
	}
	f.logs = append(f.logs, r)
}

func (f *Fake) List(ctx context.Context) ([]ports.Channel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]ports.Channel, 0, len(f.channels))
	for _, c := range f.channels {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *Fake) Get(ctx context.Context, id int) (ports.Channel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.channels[id]
	if !ok {
		return ports.Channel{}, ports.ErrNotFound
	}
	return c, nil
}

func (f *Fake) Create(ctx context.Context, in ports.ChannelCreate) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.channels {
		if c.Name == in.Name {
			return 0, fmt.Errorf("duplicate name %q", in.Name)
		}
	}
	id := f.nextID
	f.nextID++
	st := in.Status
	if st == 0 {
		st = ports.ChannelEnabled
	}
	f.channels[id] = ports.Channel{
		ID: id, Type: in.Type, Name: in.Name, Key: in.Key, BaseURL: in.BaseURL, Models: append([]string(nil), in.Models...),
		Group: in.Group, Priority: in.Priority, Weight: in.Weight, Tag: in.Tag, Status: st, TestModel: in.TestModel,
	}
	f.Writes++
	return id, nil
}

func (f *Fake) Update(ctx context.Context, id int, p ports.ChannelPatch) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.channels[id]
	if !ok {
		return ports.ErrNotFound
	}
	if p.Name != nil {
		c.Name = *p.Name
	}
	if p.Key != nil {
		c.Key = *p.Key
	}
	if p.BaseURL != nil {
		c.BaseURL = *p.BaseURL
	}
	if p.Models != nil {
		c.Models = append([]string(nil), (*p.Models)...)
	}
	if p.Group != nil {
		c.Group = *p.Group
	}
	if p.Priority != nil {
		c.Priority = *p.Priority
	}
	if p.Weight != nil {
		c.Weight = *p.Weight
	}
	if p.Tag != nil {
		c.Tag = *p.Tag
	}
	if p.Status != nil {
		c.Status = *p.Status
	}
	if p.TestModel != nil {
		c.TestModel = *p.TestModel
	}
	f.channels[id] = c
	f.Writes++
	return nil
}

func (f *Fake) Delete(ctx context.Context, id int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.channels[id]; !ok {
		return ports.ErrNotFound
	}
	delete(f.channels, id)
	f.Writes++
	return nil
}

func (f *Fake) Test(ctx context.Context, id int, model string) (ports.ChannelTestResult, error) {
	if _, err := f.Get(ctx, id); err != nil {
		return ports.ChannelTestResult{}, err
	}
	return ports.ChannelTestResult{Success: true, Duration: 0.1}, nil
}

// OptionStore.Get
func (f *Fake) getOptions(keys []string) map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]string{}
	for _, k := range keys {
		if v, ok := f.options[k]; ok {
			out[k] = v
		}
	}
	return out
}

// Set implements OptionStore.
func (f *Fake) Set(ctx context.Context, key, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.options[key] = value
	f.Writes++
	return nil
}

// Info implements Backend.
func (f *Fake) Info(ctx context.Context) (ports.BackendInfo, error) {
	return ports.BackendInfo{Mode: "fake", Version: "fake", QuotaPerUnit: 500000}, nil
}

// fakeOptions adapts Get's variadic signature (Fake already has Get for channels).
type fakeOptions Fake

func (o *fakeOptions) Get(ctx context.Context, keys ...string) (map[string]string, error) {
	return (*Fake)(o).getOptions(keys), nil
}
func (o *fakeOptions) Set(ctx context.Context, k, v string) error { return (*Fake)(o).Set(ctx, k, v) }

type fakeLogs Fake

func (l *fakeLogs) FetchAfter(ctx context.Context, q ports.LogQuery) ([]ports.LogRecord, error) {
	f := (*Fake)(l)
	f.mu.Lock()
	defer f.mu.Unlock()
	lim := q.Limit
	if lim <= 0 || lim > 500 {
		lim = 500
	}
	types := map[int]bool{}
	for _, t := range q.Types {
		types[t] = true
	}
	var out []ports.LogRecord
	for _, r := range f.logs {
		if r.ID <= q.AfterID || r.CreatedAt < q.Since {
			continue
		}
		if len(types) > 0 && !types[r.Type] {
			continue
		}
		out = append(out, r)
		if len(out) >= lim {
			break
		}
	}
	return out, nil
}

func (l *fakeLogs) Recent(ctx context.Context, ch, typ int, since int64, limit int) ([]ports.LogRecord, error) {
	f := (*Fake)(l)
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []ports.LogRecord
	for i := len(f.logs) - 1; i >= 0 && len(out) < limit; i-- {
		r := f.logs[i]
		if r.ChannelID == ch && r.Type == typ && r.CreatedAt >= since {
			out = append(out, r)
		}
	}
	return out, nil
}

func (l *fakeLogs) DeleteBefore(ctx context.Context, before int64) (int64, error) {
	f := (*Fake)(l)
	f.mu.Lock()
	defer f.mu.Unlock()
	kept := f.logs[:0]
	var n int64
	for _, r := range f.logs {
		if r.CreatedAt < before {
			n++
			continue
		}
		kept = append(kept, r)
	}
	f.logs = kept
	return n, nil
}

type fakeAuth struct{}

func (fakeAuth) Mode() string { return "fake" }

func (fakeAuth) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := strconv.Atoi(c.GetHeader("X-Fake-Role"))
		if c.GetHeader("X-Fake-Role") == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
			return
		}
		if role != 100 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "message": "仅 root 可用"})
			return
		}
		c.Set("fake.id", ports.Identity{UserID: 1, Username: "root", Role: role})
		c.Next()
	}
}

func (fakeAuth) SecureMiddleware() gin.HandlerFunc { return func(c *gin.Context) { c.Next() } }

func (fakeAuth) Identity(c *gin.Context) (ports.Identity, bool) {
	v, ok := c.Get("fake.id")
	if !ok {
		return ports.Identity{}, false
	}
	id, ok := v.(ports.Identity)
	return id, ok
}
