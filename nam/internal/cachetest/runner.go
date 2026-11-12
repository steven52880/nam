package cachetest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/steven52880/nam/internal/audit"
	"github.com/steven52880/nam/internal/crypto"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/safego"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

// Limits for the number of rounds.
const (
	DefaultRounds = 3
	MinRounds     = 2
	MaxRounds     = 10
)

// DefaultTimeout bounds one whole test.
const DefaultTimeout = 5 * time.Minute

// Runner starts cache tests in the background (safego) and persists the result.
type Runner struct {
	St       *store.Store
	Box      *crypto.Box
	Channels ports.ChannelStore
	Backend  ports.Backend   // optional: default NewAPI address
	Audit    *audit.Recorder // optional
	HTTP     *http.Client    // optional
	Timeout  time.Duration   // default DefaultTimeout
	// NewSender overrides how a round is sent (tests). Default: HTTPSender.
	NewSender func(baseURL, token string) Sender

	mu      sync.Mutex
	running map[int]uint // channel id → test id
}

// StartInput describes one test.
type StartInput struct {
	ChannelID int
	Model     string
	Rounds    int    // 0 = DefaultRounds
	Protocol  string // "" = chosen from the model name
	Mode      string // "" = store.CacheTestModeDirect; or store.CacheTestModeNewAPI
}

func (r *Runner) timeout() time.Duration {
	if r.Timeout > 0 {
		return r.Timeout
	}
	return DefaultTimeout
}

// Start validates, records a running row and launches the test. It returns the test id.
// Errors: 400 invalid input / channel missing or disabled / not configured; 409 already running.
func (r *Runner) Start(ctx context.Context, in StartInput) (uint, error) {
	if in.Model == "" {
		return 0, resp.Invalid("model 必填")
	}
	if in.Rounds == 0 {
		in.Rounds = DefaultRounds
	}
	if in.Rounds < MinRounds || in.Rounds > MaxRounds {
		return 0, resp.Invalid(fmt.Sprintf("轮数必须在 %d~%d 之间", MinRounds, MaxRounds))
	}
	switch in.Protocol {
	case "":
		in.Protocol = ProtocolFor(in.Model)
	case ProtocolAnthropic, ProtocolOpenAI:
	default:
		return 0, resp.Invalid("protocol 只能是 anthropic 或 openai")
	}

	switch in.Mode {
	case "":
		in.Mode = store.CacheTestModeDirect
	case store.CacheTestModeDirect, store.CacheTestModeNewAPI:
	default:
		return 0, resp.Invalid("mode 只能是 direct 或 newapi")
	}

	ch, err := r.Channels.Get(ctx, in.ChannelID)
	if err != nil {
		if resp.Status(err) == http.StatusNotFound {
			return 0, resp.Invalid("渠道不存在")
		}
		return 0, resp.Upstream(err)
	}
	var baseURL, secret string
	if in.Mode == store.CacheTestModeNewAPI {
		if ch.Status != ports.ChannelEnabled {
			return 0, resp.Invalid("渠道未启用：NewAPI 只会向启用状态的渠道分发请求，请先启用该渠道")
		}
		if secret, err = LoadToken(ctx, r.St, r.Box); err != nil {
			return 0, err
		}
		baseURL = ResolveBaseURL(ctx, r.St, r.Backend)
	} else if baseURL, secret, err = ResolveDirect(ctx, r.St, r.Box, in.ChannelID); err != nil {
		return 0, err
	}

	r.mu.Lock()
	if _, busy := r.running[in.ChannelID]; busy {
		r.mu.Unlock()
		return 0, resp.Conflict("该渠道已有缓存率测试正在进行")
	}
	if r.running == nil {
		r.running = map[int]uint{}
	}
	r.running[in.ChannelID] = 0
	r.mu.Unlock()

	row := store.CacheTest{
		ChannelID: in.ChannelID, Model: in.Model, Protocol: in.Protocol, Mode: in.Mode,
		Status: store.CacheTestRunning, Rounds: in.Rounds, StartedAt: time.Now().Unix(),
	}
	if err := r.St.CreateCacheTest(ctx, &row); err != nil {
		r.release(in.ChannelID)
		return 0, err
	}
	r.mu.Lock()
	r.running[in.ChannelID] = row.ID
	r.mu.Unlock()

	sender := r.sender(in.Mode, baseURL, secret)
	// Keep operator/trigger values for the audit row, but outlive the HTTP request.
	base := context.WithoutCancel(ctx)
	safego.Go("cachetest", func() {
		defer r.release(in.ChannelID)
		r.execute(base, row, sender)
	})
	return row.ID, nil
}

func (r *Runner) release(channelID int) {
	r.mu.Lock()
	delete(r.running, channelID)
	r.mu.Unlock()
}

// sender builds the Sender of a mode. In direct mode secret is the upstream key; in
// newapi mode it is the NewAPI test token.
func (r *Runner) sender(mode, baseURL, secret string) Sender {
	if r.NewSender != nil {
		return r.NewSender(baseURL, secret)
	}
	if mode == store.CacheTestModeNewAPI {
		return &HTTPSender{BaseURL: baseURL, Token: secret, Client: r.HTTP}
	}
	return &DirectSender{BaseURL: baseURL, Key: secret, Client: r.HTTP}
}

// execute runs all rounds, then stores the result and writes the audit row.
func (r *Runner) execute(base context.Context, row store.CacheTest, sender Sender) {
	t0 := time.Now()
	runCtx, cancel := context.WithTimeout(base, r.timeout())
	defer cancel()

	var rounds []Usage
	runErr := r.runRounds(runCtx, row, sender, &rounds)

	fields := map[string]any{"duration_ms": time.Since(t0).Milliseconds()}
	if b, err := json.Marshal(rounds); err == nil {
		fields["per_round"] = string(b)
	}
	tot := SumRounds(rounds)
	fields["input_tokens"], fields["cache_read_tokens"], fields["cache_write_tokens"] = tot.Input, tot.CacheRead, tot.CacheWrite
	var rate *float64
	switch {
	case runErr != nil:
		fields["status"], fields["error"], fields["rate"] = store.CacheTestFail, runErr.Error(), nil // drop provisional rate
	default:
		if rate = Rate(rounds); rate == nil {
			runErr = fmt.Errorf("上游没有返回输入 token 用量，无法计算缓存率")
			fields["status"], fields["error"], fields["rate"] = store.CacheTestFail, runErr.Error(), nil
		} else {
			fields["status"], fields["rate"] = store.CacheTestOK, *rate
		}
	}
	if err := r.St.UpdateCacheTest(base, row.ID, fields); err != nil {
		slog.Error("nam: save cache test", "id", row.ID, "err", err)
	}
	r.record(base, row, rounds, rate, runErr)
}

func (r *Runner) runRounds(ctx context.Context, row store.CacheTest, sender Sender, out *[]Usage) error {
	var history []Turn
	for i := 1; i <= row.Rounds; i++ {
		body, err := BuildRound(row.Protocol, row.Model, i, history)
		if err != nil {
			return fmt.Errorf("第 %d 轮构造请求失败：%w", i, err)
		}
		t0 := time.Now()
		res, err := sender.Send(ctx, Request{ChannelID: row.ChannelID, Protocol: row.Protocol, Body: body})
		if err != nil {
			return fmt.Errorf("第 %d 轮失败：%w", i, err)
		}
		res.Usage.Round = i
		res.Usage.DurationMs = time.Since(t0).Milliseconds()
		*out = append(*out, res.Usage)
		history = append(history, Turn{Question: QuestionFor(i), Answer: res.Answer})
		r.saveProgress(ctx, row.ID, *out)
	}
	return nil
}

// saveProgress persists the rounds finished so far (live view). The status stays "running";
// rate is provisional and nil until round 2 has finished.
func (r *Runner) saveProgress(ctx context.Context, id uint, rounds []Usage) {
	b, err := json.Marshal(rounds)
	if err != nil {
		return
	}
	tot := SumRounds(rounds)
	var rate any
	if v := Rate(rounds); v != nil {
		rate = *v
	}
	fields := map[string]any{
		"per_round": string(b), "rate": rate,
		"input_tokens": tot.Input, "cache_read_tokens": tot.CacheRead, "cache_write_tokens": tot.CacheWrite,
	}
	if err := r.St.UpdateCacheTest(ctx, id, fields); err != nil {
		slog.Error("nam: save cache test progress", "id", id, "err", err)
	}
}

func (r *Runner) record(ctx context.Context, row store.CacheTest, rounds []Usage, rate *float64, runErr error) {
	if r.Audit == nil {
		return
	}
	e := store.AuditLog{
		Kind: store.AuditCacheTest, Target: fmt.Sprintf("channel:%d", row.ChannelID), Action: "cache_test",
		Request: audit.JSON(map[string]any{"model": row.Model, "rounds": row.Rounds, "protocol": row.Protocol, "mode": row.Mode}),
		Result:  store.ResultOK,
	}
	via := "直连上游"
	if row.Mode == store.CacheTestModeNewAPI {
		via = "经 NewAPI"
	}
	if runErr != nil {
		e.Result, e.Error = store.ResultFail, runErr.Error()
		e.Summary = fmt.Sprintf("手动缓存率测试失败（%s，渠道 %d，%s）", via, row.ChannelID, row.Model)
	} else {
		e.Summary = fmt.Sprintf("手动缓存率测试 %.1f%%（%s，渠道 %d，%s，%d 轮）", *rate*100, via, row.ChannelID, row.Model, row.Rounds)
	}
	e.Response = audit.JSON(map[string]any{"test_id": row.ID, "rate": rate, "per_round": rounds})
	r.Audit.Record(ctx, e)
}

// Get returns one run. A "running" row nobody is executing (process restarted) is
// reported as failed so the UI stops polling.
func (r *Runner) Get(ctx context.Context, id uint) (store.CacheTest, error) {
	t, err := r.St.GetCacheTest(ctx, id)
	if err != nil {
		return t, err
	}
	if t.Status == store.CacheTestRunning && !r.isRunning(t.ChannelID, t.ID) {
		t.Status, t.Error = store.CacheTestFail, "测试被中断（nam 重启或超时）"
	}
	return t, nil
}

func (r *Runner) isRunning(channelID int, id uint) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.running[channelID]
	return ok && cur == id
}

// Latest returns the newest run of every channel.
func (r *Runner) Latest(ctx context.Context) ([]store.CacheTest, error) {
	ts, err := r.St.LatestCacheTests(ctx)
	if err != nil {
		return nil, err
	}
	for i := range ts {
		if ts[i].Status == store.CacheTestRunning && !r.isRunning(ts[i].ChannelID, ts[i].ID) {
			ts[i].Status, ts[i].Error = store.CacheTestFail, "测试被中断（nam 重启或超时）"
		}
	}
	if ts == nil {
		ts = []store.CacheTest{}
	}
	return ts, nil
}
