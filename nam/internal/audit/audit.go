// Package audit records writes and decorates ports so every NewAPI write is audited
// identically in both run modes.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
	"gorm.io/gorm"
)

type ctxKey int

const (
	keyOperator ctxKey = iota
	keyTrigger
)

// WithOperator tags ctx with the operator name (written to audit_log.operator).
func WithOperator(ctx context.Context, op string) context.Context {
	return context.WithValue(ctx, keyOperator, op)
}

// WithTrigger tags ctx as auto (scheduler) or manual (user action).
func WithTrigger(ctx context.Context, trig string) context.Context {
	return context.WithValue(ctx, keyTrigger, trig)
}

func operator(ctx context.Context) string {
	if v, ok := ctx.Value(keyOperator).(string); ok {
		return v
	}
	return "system"
}

func trigger(ctx context.Context) string {
	if v, ok := ctx.Value(keyTrigger).(string); ok {
		return v
	}
	return store.TriggerAuto
}

// Recorder writes audit_log rows.
type Recorder struct {
	db *gorm.DB
}

// New returns a Recorder.
func New(db *gorm.DB) *Recorder { return &Recorder{db: db} }

// Record inserts a row, filling At/Operator/Trigger when empty. Errors are logged, not returned,
// so a failing audit write never blocks the operation that already happened.
func (r *Recorder) Record(ctx context.Context, e store.AuditLog) uint {
	if e.At == 0 {
		e.At = time.Now().Unix()
	}
	if e.Operator == "" {
		e.Operator = operator(ctx)
	}
	if e.Trigger == "" {
		e.Trigger = trigger(ctx)
	}
	if e.Result == "" {
		e.Result = store.ResultOK
	}
	if err := r.db.WithContext(context.WithoutCancel(ctx)).Create(&e).Error; err != nil {
		slog.Error("nam: audit write failed", "err", err, "target", e.Target)
		return 0
	}
	return e.ID
}

// JSON marshals v for Before/After/Request columns.
func JSON(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

func resultOf(err error) (string, string) {
	if err != nil {
		return store.ResultFail, err.Error()
	}
	return store.ResultOK, ""
}

// Channels wraps a ChannelStore; writes are recorded with the prior value.
func (r *Recorder) Channels(inner ports.ChannelStore) ports.ChannelStore {
	return &auditedChannels{inner: inner, r: r}
}

// Options wraps an OptionStore; writes are recorded with the prior value.
func (r *Recorder) Options(inner ports.OptionStore) ports.OptionStore {
	return &auditedOptions{inner: inner, r: r}
}

// Models wraps a ModelFetcher. Fetching is read-only, so nothing is recorded and the key
// is never logged; a nil fetcher stays nil so callers can keep testing for presence.
func (r *Recorder) Models(inner ports.ModelFetcher) ports.ModelFetcher {
	if inner == nil {
		return nil
	}
	return &auditedModels{inner: inner}
}

type auditedModels struct {
	inner ports.ModelFetcher
}

func (a *auditedModels) FetchModels(ctx context.Context, baseURL string, channelType int, key string) ([]string, error) {
	return a.inner.FetchModels(ctx, baseURL, channelType, key)
}

type auditedChannels struct {
	inner ports.ChannelStore
	r     *Recorder
}

func (a *auditedChannels) List(ctx context.Context) ([]ports.Channel, error) {
	return a.inner.List(ctx)
}
func (a *auditedChannels) Get(ctx context.Context, id int) (ports.Channel, error) {
	return a.inner.Get(ctx, id)
}

func (a *auditedChannels) Create(ctx context.Context, ch ports.ChannelCreate) (int, error) {
	id, err := a.inner.Create(ctx, ch)
	res, msg := resultOf(err)
	redacted := ch
	redacted.Key = redactKey(ch.Key)
	a.r.Record(ctx, store.AuditLog{
		Kind: store.AuditChannel, Target: fmt.Sprintf("channel:%d", id), Action: "create",
		Summary: "新建渠道 " + ch.Name, After: JSON(redacted), Request: JSON(redacted),
		Result: res, Error: msg,
	})
	return id, err
}

func (a *auditedChannels) Update(ctx context.Context, id int, p ports.ChannelPatch) error {
	before, berr := a.inner.Get(ctx, id)
	err := a.inner.Update(ctx, id, p)
	res, msg := resultOf(err)
	rp := p
	if p.Key != nil {
		k := redactKey(*p.Key)
		rp.Key = &k
	}
	e := store.AuditLog{
		Kind: store.AuditChannel, Target: fmt.Sprintf("channel:%d", id), Action: "update",
		Summary: "修改渠道 " + before.Name, After: JSON(rp), Request: JSON(rp), Result: res, Error: msg,
	}
	if berr == nil {
		before.Key = ""
		e.Before = JSON(before)
	}
	a.r.Record(ctx, e)
	return err
}

func (a *auditedChannels) Delete(ctx context.Context, id int) error {
	before, berr := a.inner.Get(ctx, id)
	err := a.inner.Delete(ctx, id)
	res, msg := resultOf(err)
	e := store.AuditLog{
		Kind: store.AuditChannel, Target: fmt.Sprintf("channel:%d", id), Action: "delete",
		Summary: "删除渠道 " + before.Name, Result: res, Error: msg,
	}
	if berr == nil {
		before.Key = ""
		e.Before = JSON(before)
	}
	a.r.Record(ctx, e)
	return err
}

func (a *auditedChannels) Test(ctx context.Context, id int, model string) (ports.ChannelTestResult, error) {
	return a.inner.Test(ctx, id, model)
}

type auditedOptions struct {
	inner ports.OptionStore
	r     *Recorder
}

func (a *auditedOptions) Get(ctx context.Context, keys ...string) (map[string]string, error) {
	return a.inner.Get(ctx, keys...)
}

func (a *auditedOptions) Set(ctx context.Context, key, value string) error {
	var before string
	if m, err := a.inner.Get(ctx, key); err == nil {
		before = m[key]
	}
	err := a.inner.Set(ctx, key, value)
	res, msg := resultOf(err)
	a.r.Record(ctx, store.AuditLog{
		Kind: kindForOption(key), Target: "option:" + key, Action: "set",
		Summary: "写入 NewAPI 配置 " + key, Before: before, After: value, Result: res, Error: msg,
	})
	return err
}

func kindForOption(key string) string {
	switch key {
	case "GroupRatio", "ModelRatio", "CompletionRatio", "CacheRatio", "CreateCacheRatio", "ModelPrice":
		return store.AuditPrice
	}
	return store.AuditSetting
}

func redactKey(k string) string {
	if k == "" {
		return ""
	}
	if len(k) <= 8 {
		return "***"
	}
	return k[:4] + "…" + k[len(k)-4:]
}
