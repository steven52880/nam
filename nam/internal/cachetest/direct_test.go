package cachetest

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

const upstreamKey = "sk-UPSTREAMKEY999"

// linkUpstream gives the env's channel an upstream (BaseURL = baseURL) with a sealed token.
func (e *env) linkUpstream(t *testing.T, baseURL string) {
	t.Helper()
	ctx := context.Background()
	up := store.Upstream{Name: "u1", Type: store.UpstreamNewAPI, BaseURL: baseURL, CurrencyRate: 1}
	if err := e.st.DB.Create(&up).Error; err != nil {
		t.Fatal(err)
	}
	sealed, err := e.r.Box.Seal([]byte(upstreamKey))
	if err != nil {
		t.Fatal(err)
	}
	tok := store.UpstreamToken{UpstreamID: up.ID, Name: "t1", Key: sealed}
	if err := e.st.DB.Create(&tok).Error; err != nil {
		t.Fatal(err)
	}
	ext := store.ChannelExt{ChannelID: e.ch, UpstreamID: &up.ID, TokenID: &tok.ID, Discount: 1}
	if err := e.st.DB.WithContext(ctx).Create(&ext).Error; err != nil {
		t.Fatal(err)
	}
}

func TestDirectAnthropicDisabledChannelNoNewAPIToken(t *testing.T) {
	e := newEnv(t, false, ports.ChannelManualDisabled) // disabled + no NewAPI test token
	e.linkUpstream(t, e.srv.URL+"/")
	e.api.reply = func(n int) (int, string) {
		if n == 1 {
			return 200, anthropicBody(10, 0, 1000)
		}
		return 200, anthropicBody(20, 1000, 0)
	}
	id, err := e.r.Start(context.Background(), StartInput{ChannelID: e.ch, Model: "claude-sonnet-4", Rounds: 2})
	if err != nil {
		t.Fatal(err)
	}
	got := e.wait(t, id)
	if got.Status != store.CacheTestOK || got.Mode != store.CacheTestModeDirect || got.Rate == nil || !near(*got.Rate, 1000.0/1020.0) {
		t.Fatalf("row = %+v", got)
	}
	for i, q := range e.api.reqs {
		if q.path != "/v1/messages" || q.auth != "Bearer "+upstreamKey || q.xkey != upstreamKey {
			t.Fatalf("req %d: %+v", i, q)
		}
	}
	var logs []store.AuditLog
	e.st.DB.Where("kind = ?", store.AuditCacheTest).Find(&logs)
	if len(logs) != 1 || !strings.Contains(logs[0].Summary, "直连上游") ||
		strings.Contains(logs[0].Request+logs[0].Response+logs[0].Summary+logs[0].Error, "UPSTREAMKEY") {
		t.Fatalf("audit = %+v", logs)
	}
}

func TestDirectOpenAIBaseWithV1NotDoubled(t *testing.T) {
	e := newEnv(t, false, ports.ChannelEnabled)
	e.linkUpstream(t, e.srv.URL+"/v1/")
	e.api.reply = func(n int) (int, string) {
		if n == 1 {
			return 200, openaiBody(1000, 0)
		}
		return 200, openaiBody(1000, 800)
	}
	id, err := e.r.Start(context.Background(), StartInput{ChannelID: e.ch, Model: "gpt-5", Rounds: 2})
	if err != nil {
		t.Fatal(err)
	}
	got := e.wait(t, id)
	if got.Status != store.CacheTestOK || got.Rate == nil || !near(*got.Rate, 0.8) {
		t.Fatalf("row = %+v", got)
	}
	q := e.api.reqs[0]
	if q.path != "/v1/chat/completions" || q.auth != "Bearer "+upstreamKey || q.xkey != "" {
		t.Fatalf("req = %+v", q)
	}
}

func TestDirectRejectsChannelWithoutExt(t *testing.T) {
	e := newEnv(t, true, ports.ChannelEnabled)
	_, err := e.r.Start(context.Background(), StartInput{ChannelID: e.ch, Model: "gpt-5"})
	if !isInvalid(err) || !strings.Contains(err.Error(), "无法直连上游测试") {
		t.Fatalf("err = %v", err)
	}
	if _, err := e.r.Start(context.Background(), StartInput{ChannelID: e.ch, Model: "gpt-5", Mode: "x"}); !isInvalid(err) {
		t.Fatalf("bad mode: %v", err)
	}
}

func TestDirectProgressPersistedWhileRunning(t *testing.T) {
	e := newEnv(t, false, ports.ChannelEnabled)
	e.linkUpstream(t, e.srv.URL)
	e.api.gate, e.api.gateFrom = make(chan struct{}), 2
	e.api.reply = func(n int) (int, string) {
		if n == 1 {
			return 200, openaiBody(1000, 0)
		}
		return 200, openaiBody(1000, 900)
	}
	id, err := e.r.Start(context.Background(), StartInput{ChannelID: e.ch, Model: "gpt-5", Rounds: 3})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		row, err := e.st.GetCacheTest(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		var rounds []Usage
		_ = json.Unmarshal([]byte(row.PerRound), &rounds)
		if len(rounds) == 1 {
			if row.Status != store.CacheTestRunning || row.Rate != nil || rounds[0].Round != 1 {
				t.Fatalf("live row = %+v", row)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("round 1 never persisted: %+v", row)
		}
		time.Sleep(20 * time.Millisecond)
	}
	close(e.api.gate)
	got := e.wait(t, id)
	var rounds []Usage
	if err := json.Unmarshal([]byte(got.PerRound), &rounds); err != nil || len(rounds) != 3 {
		t.Fatalf("final rounds = %q (%v)", got.PerRound, err)
	}
	if got.Status != store.CacheTestOK || got.Rate == nil || !near(*got.Rate, 0.9) {
		t.Fatalf("final = %+v", got)
	}
}

func TestDirectUnauthorizedClearsProvisionalRateAndHidesKey(t *testing.T) {
	e := newEnv(t, false, ports.ChannelEnabled)
	e.linkUpstream(t, e.srv.URL)
	e.api.reply = func(n int) (int, string) {
		if n <= 2 {
			return 200, openaiBody(1000, 500*(n-1))
		}
		return 401, `{"error":{"message":"bad key ` + upstreamKey + `"}}`
	}
	id, err := e.r.Start(context.Background(), StartInput{ChannelID: e.ch, Model: "gpt-5", Rounds: 3})
	if err != nil {
		t.Fatal(err)
	}
	got := e.wait(t, id)
	if got.Status != store.CacheTestFail || got.Rate != nil {
		t.Fatalf("row = %+v", got)
	}
	if !strings.Contains(got.Error, "上游拒绝了令牌（HTTP 401）") || strings.Contains(got.Error, "UPSTREAMKEY") {
		t.Fatalf("error = %q", got.Error)
	}
	var rounds []Usage
	if err := json.Unmarshal([]byte(got.PerRound), &rounds); err != nil || len(rounds) != 2 {
		t.Fatalf("per_round = %q", got.PerRound)
	}
}

func TestDirectConnectionErrorNamesHostOnly(t *testing.T) {
	e := newEnv(t, false, ports.ChannelEnabled)
	e.linkUpstream(t, "http://127.0.0.1:1")
	id, err := e.r.Start(context.Background(), StartInput{ChannelID: e.ch, Model: "gpt-5", Rounds: 2})
	if err != nil {
		t.Fatal(err)
	}
	got := e.wait(t, id)
	if got.Status != store.CacheTestFail || !strings.Contains(got.Error, "连不上上游 127.0.0.1:1") {
		t.Fatalf("row = %+v", got)
	}
}
