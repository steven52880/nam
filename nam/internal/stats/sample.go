// Package stats aggregates NewAPI consume/error logs into nam.db stats_hourly and
// computes channel metrics (success rate, latency, speed, cache hit, cost).
package stats

import (
	"encoding/json"
	"strings"

	"github.com/steven52880/nam/ports"
)

// TestTokenName is the token_name NewAPI writes for channel tests; those logs are skipped.
const TestTokenName = "模型测试"

// Error classes (keys of fail_by_class).
const (
	ClassTimeout = "timeout"
	Class429     = "429"
	Class401     = "401"
	ClassBalance = "balance"
	Class5xx     = "5xx"
	ClassOther   = "other"
)

// Sample is one parsed log, normalised for aggregation.
type Sample struct {
	ChannelID int
	Model     string
	Group     string
	CreatedAt int64
	Hour      int64 // CreatedAt truncated to the hour (unix seconds)

	OK        bool   // consume = true, error = false
	FailClass string // set when !OK

	Input  int64 // non-cached input
	Read   int64 // cache read
	W5     int64 // cache write 5m (incl. unsplit cache_creation_tokens)
	W1     int64 // cache write 1h
	Output int64

	FRT      float64 // first response time ms; 0 = no sample
	Speed    float64 // token/s; 0 = no sample
	SpeedEst bool    // Speed was estimated from whole-second use_time
	Quota    int64

	UseTimeMS bool // the log carried other.use_time_ms (patch P1)
}

// logOther is the subset of NewAPI's logs.other JSON nam reads.
type logOther struct {
	CacheTokens   float64  `json:"cache_tokens"`
	CacheCreation float64  `json:"cache_creation_tokens"`
	CacheWrite5m  float64  `json:"cache_creation_tokens_5m"`
	CacheWrite1h  float64  `json:"cache_creation_tokens_1h"`
	UsageSemantic string   `json:"usage_semantic"`
	FRT           float64  `json:"frt"`
	UseTimeMS     *float64 `json:"use_time_ms"`
	StatusCode    float64  `json:"status_code"`
}

// Parse normalises one log. ok=false means the log is not counted (wrong type,
// channel test, no channel).
func Parse(r ports.LogRecord) (Sample, bool) {
	if r.Type != ports.LogTypeConsume && r.Type != ports.LogTypeError {
		return Sample{}, false
	}
	if r.TokenName == TestTokenName || r.ChannelID <= 0 {
		return Sample{}, false
	}
	var o logOther
	if r.Other != "" {
		// Partial decode is fine: mistyped or missing fields stay 0.
		_ = json.Unmarshal([]byte(r.Other), &o)
	}
	s := Sample{
		ChannelID: r.ChannelID, Model: r.ModelName, Group: r.Group,
		CreatedAt: r.CreatedAt, Hour: r.CreatedAt - r.CreatedAt%3600,
		Quota: int64(r.Quota),
	}
	if r.Type == ports.LogTypeError {
		s.FailClass = classify(int(o.StatusCode), r.Content)
		return s, true
	}
	s.OK = true
	s.UseTimeMS = o.UseTimeMS != nil
	s.Read = nonNeg(int64(o.CacheTokens))
	s.W1 = nonNeg(int64(o.CacheWrite1h))
	w5 := nonNeg(int64(o.CacheWrite5m))
	s.W5 = w5 + nonNeg(int64(o.CacheCreation)-w5-s.W1)
	prompt := int64(r.PromptTokens)
	if o.UsageSemantic == "anthropic" {
		s.Input = nonNeg(prompt)
	} else {
		s.Input = nonNeg(prompt - s.Read - s.W5 - s.W1)
	}
	s.Output = nonNeg(int64(r.CompletionTokens))
	if r.IsStream && o.FRT > 0 {
		s.FRT = o.FRT
		if s.Output > 0 {
			s.Speed, s.SpeedEst = speed(s.Output, o.FRT, o.UseTimeMS, r.UseTime)
		}
	}
	return s, true
}

// speed returns token/s; 0 when the generation time is not positive.
func speed(output int64, frt float64, useTimeMS *float64, useTimeS int) (float64, bool) {
	total, est := float64(useTimeS)*1000, true
	if useTimeMS != nil {
		total, est = *useTimeMS, false
	}
	gen := (total - frt) / 1000
	if gen <= 0 {
		return 0, false
	}
	return float64(output) / gen, est
}

// classify picks the first matching error class (plan order).
func classify(status int, content string) string {
	c := strings.ToLower(content)
	switch {
	case status == 408 || status == 504 || containsAny(c, "timeout", "deadline exceeded", "超时"):
		return ClassTimeout
	case status == 429:
		return Class429
	case status == 401 || status == 403:
		return Class401
	case status == 402 || containsAny(c, "insufficient_quota", "余额不足", "额度不足", "insufficient balance", "credit"):
		return ClassBalance
	case status >= 500 && status <= 599:
		return Class5xx
	default:
		return ClassOther
	}
}

func containsAny(s string, subs ...string) bool {
	for _, x := range subs {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}

func nonNeg(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}
