package cachetest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/steven52880/nam/internal/audit"
	"github.com/steven52880/nam/internal/crypto"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
	"github.com/steven52880/nam/ports/portstest"
)

const testToken = "sk-SECRETTOKEN123"

type seen struct {
	path, auth, xkey string
	body             string
}

// fakeNewAPI answers every round with reply(roundIndex) (1-based) and records requests.
type fakeNewAPI struct {
	mu    sync.Mutex
	reqs  []seen
	reply func(n int) (int, string)
	gate  chan struct{} // when non-nil, requests with index >= gateFrom wait for it to close
	// gateFrom is the 1-based request index from which gate applies (0 = all requests).
	gateFrom int
}

func (f *fakeNewAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.reqs = append(f.reqs, seen{r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("x-api-key"), string(b)})
	n := len(f.reqs)
	gate := f.gate
	f.mu.Unlock()
	if gate != nil && n >= f.gateFrom {
		<-gate
	}
	code, body := f.reply(n)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = io.WriteString(w, body)
}

func anthropicBody(in, read, write int) string {
	return fmt.Sprintf(`{"content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":%d,"output_tokens":5,"cache_read_input_tokens":%d,"cache_creation_input_tokens":%d}}`, in, read, write)
}

func openaiBody(prompt, cached int) string {
	return fmt.Sprintf(`{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":%d,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":%d}}}`, prompt, cached)
}

type env struct {
	r   *Runner
	st  *store.Store
	fk  *portstest.Fake
	api *fakeNewAPI
	srv *httptest.Server
	ch  int
}

func newEnv(t *testing.T, withToken bool, status int) *env {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "nam.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	box, err := crypto.New("k")
	if err != nil {
		t.Fatal(err)
	}
	fk := portstest.NewFake()
	ch, err := fk.Create(context.Background(), ports.ChannelCreate{Name: "c1", Status: status})
	if err != nil {
		t.Fatal(err)
	}
	api := &fakeNewAPI{}
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	ctx := context.Background()
	if err := st.SetKV(ctx, store.KeyNewAPIURL, srv.URL); err != nil {
		t.Fatal(err)
	}
	if withToken {
		if err := SaveToken(ctx, st, box, testToken); err != nil {
			t.Fatal(err)
		}
	}
	r := &Runner{St: st, Box: box, Channels: fk, Backend: fk, Audit: audit.New(st.DB), Timeout: 20 * time.Second}
	return &env{r: r, st: st, fk: fk, api: api, srv: srv, ch: ch}
}

func (e *env) wait(t *testing.T, id uint) store.CacheTest {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		got, err := e.r.St.GetCacheTest(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != store.CacheTestRunning {
			return got
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("test did not finish")
	return store.CacheTest{}
}

func TestRunnerAnthropicRoundsRateAndKey(t *testing.T) {
	e := newEnv(t, true, ports.ChannelEnabled)
	e.api.reply = func(n int) (int, string) {
		switch n {
		case 1:
			return 200, anthropicBody(10, 0, 1000)
		case 2:
			return 200, anthropicBody(20, 1000, 0)
		}
		return 200, anthropicBody(30, 1000, 0)
	}
	id, err := e.r.Start(context.Background(), StartInput{ChannelID: e.ch, Model: "Claude-Sonnet-4", Mode: store.CacheTestModeNewAPI})
	if err != nil {
		t.Fatal(err)
	}
	got := e.wait(t, id)
	if got.Status != store.CacheTestOK || got.Rounds != 3 || got.Protocol != ProtocolAnthropic {
		t.Fatalf("row = %+v", got)
	}
	if got.Rate == nil || !near(*got.Rate, 2000.0/2050.0) || got.InputTokens != 2050 || got.CacheReadTokens != 2000 {
		t.Fatalf("rate/sums wrong: %+v", got)
	}
	if len(e.api.reqs) != 3 {
		t.Fatalf("requests = %d", len(e.api.reqs))
	}
	wantKey := fmt.Sprintf("sk-SECRETTOKEN123-%d", e.ch)
	for i, q := range e.api.reqs {
		if q.path != "/v1/messages" || q.auth != "Bearer "+wantKey || q.xkey != wantKey {
			t.Fatalf("req %d: %+v", i, q)
		}
		if !strings.Contains(q.body, `"cache_control":{"type":"ephemeral"}`) {
			t.Fatalf("req %d lacks cache_control", i)
		}
	}
	if !strings.Contains(e.api.reqs[2].body, `"role":"assistant"`) {
		t.Fatal("round 3 must carry history")
	}
	var logs []store.AuditLog
	e.st.DB.Where("kind = ?", store.AuditCacheTest).Find(&logs)
	if len(logs) != 1 || logs[0].Result != store.ResultOK || strings.Contains(logs[0].Request+logs[0].Response+logs[0].Summary, "SECRETTOKEN") {
		t.Fatalf("audit = %+v", logs)
	}
}

func TestRunnerOpenAIProtocol(t *testing.T) {
	e := newEnv(t, true, ports.ChannelEnabled)
	e.api.reply = func(n int) (int, string) {
		if n == 1 {
			return 200, openaiBody(1000, 0)
		}
		return 200, openaiBody(1000, 900)
	}
	id, err := e.r.Start(context.Background(), StartInput{ChannelID: e.ch, Model: "gpt-5", Rounds: 2, Mode: store.CacheTestModeNewAPI})
	if err != nil {
		t.Fatal(err)
	}
	got := e.wait(t, id)
	if got.Status != store.CacheTestOK || got.Rate == nil || !near(*got.Rate, 0.9) {
		t.Fatalf("row = %+v", got)
	}
	if q := e.api.reqs[0]; q.path != "/v1/chat/completions" || strings.Contains(q.body, "cache_control") {
		t.Fatalf("req = %+v", q)
	}
}

func TestRunnerRoundFailureStopsAndHidesToken(t *testing.T) {
	e := newEnv(t, true, ports.ChannelEnabled)
	e.api.reply = func(n int) (int, string) {
		if n == 1 {
			return 200, openaiBody(1000, 0)
		}
		return 403, `{"error":{"message":"普通用户不支持指定渠道 sk-SECRETTOKEN123"}}`
	}
	id, err := e.r.Start(context.Background(), StartInput{ChannelID: e.ch, Model: "gpt-5", Rounds: 4, Mode: store.CacheTestModeNewAPI})
	if err != nil {
		t.Fatal(err)
	}
	got := e.wait(t, id)
	if got.Status != store.CacheTestFail || got.Rate != nil {
		t.Fatalf("row = %+v", got)
	}
	if !strings.Contains(got.Error, "第 2 轮失败") || !strings.Contains(got.Error, "管理员") || strings.Contains(got.Error, "SECRETTOKEN") {
		t.Fatalf("error = %q", got.Error)
	}
	if len(e.api.reqs) != 2 {
		t.Fatalf("must stop after failing round, requests = %d", len(e.api.reqs))
	}
}

func TestRunnerConfigAndInputErrors(t *testing.T) {
	ctx := context.Background()
	noTok := newEnv(t, false, ports.ChannelEnabled)
	if _, err := noTok.r.Start(ctx, StartInput{ChannelID: noTok.ch, Model: "m", Mode: store.CacheTestModeNewAPI}); !errors.Is(err, ErrNoToken) {
		t.Fatalf("no token: %v", err)
	}
	dis := newEnv(t, true, ports.ChannelManualDisabled)
	if _, err := dis.r.Start(ctx, StartInput{ChannelID: dis.ch, Model: "m", Mode: store.CacheTestModeNewAPI}); !isInvalid(err) || !strings.Contains(err.Error(), "未启用") {
		t.Fatalf("disabled: %v", err)
	}
	if _, err := dis.r.Start(ctx, StartInput{ChannelID: 999, Model: "m"}); !isInvalid(err) {
		t.Fatalf("missing channel: %v", err)
	}
	ok := newEnv(t, true, ports.ChannelEnabled)
	for _, in := range []StartInput{{ChannelID: ok.ch}, {ChannelID: ok.ch, Model: "m", Rounds: 1}, {ChannelID: ok.ch, Model: "m", Rounds: 11}, {ChannelID: ok.ch, Model: "m", Protocol: "x"}} {
		if _, err := ok.r.Start(ctx, in); !isInvalid(err) {
			t.Errorf("%+v: %v", in, err)
		}
	}
}

func TestRunnerRejectsConcurrentTestOnSameChannel(t *testing.T) {
	e := newEnv(t, true, ports.ChannelEnabled)
	e.api.gate = make(chan struct{})
	e.api.reply = func(int) (int, string) { return 200, openaiBody(100, 50) }
	id, err := e.r.Start(context.Background(), StartInput{ChannelID: e.ch, Model: "gpt-5", Rounds: 2, Mode: store.CacheTestModeNewAPI})
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.r.Start(context.Background(), StartInput{ChannelID: e.ch, Model: "gpt-5", Mode: store.CacheTestModeNewAPI})
	var ce *resp.ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("want conflict, got %v", err)
	}
	if g, _ := e.r.Get(context.Background(), id); g.Status != store.CacheTestRunning {
		t.Fatalf("status = %s", g.Status)
	}
	close(e.api.gate)
	if got := e.wait(t, id); got.Status != store.CacheTestOK {
		t.Fatalf("row = %+v", got)
	}
	// slot is released after the run
	deadline := time.Now().Add(5 * time.Second)
	for e.r.isRunning(e.ch, id) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if e.r.isRunning(e.ch, id) {
		t.Fatal("running slot not released")
	}
}

func TestResolveBaseURLPriority(t *testing.T) {
	ctx := context.Background()
	st, _ := store.Open(filepath.Join(t.TempDir(), "nam.db"))
	t.Cleanup(func() { _ = st.Close() })
	if got := ResolveBaseURL(ctx, st, portstest.NewFake()); got != DefaultEmbeddedURL {
		t.Fatalf("default = %q", got)
	}
	_ = st.SetKV(ctx, store.KeyNewAPIURL, "http://x.example:9/")
	if got := ResolveBaseURL(ctx, st, nil); got != "http://x.example:9" {
		t.Fatalf("setting = %q", got)
	}
}

func isInvalid(err error) bool {
	var ve *resp.ValidationError
	return errors.As(err, &ve)
}
