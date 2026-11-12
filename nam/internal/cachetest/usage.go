package cachetest

import (
	"encoding/json"
	"errors"
	"strings"
)

// Usage is one round's token usage, normalized across both protocols.
// Input is the TOTAL input, cached parts included:
//   - Anthropic: input_tokens + cache_read_input_tokens + cache_creation_input_tokens
//     (the API's input_tokens excludes cached tokens).
//   - OpenAI: prompt_tokens (already includes prompt_tokens_details.cached_tokens).
type Usage struct {
	Round      int   `json:"round"`
	Input      int64 `json:"input"`
	CacheRead  int64 `json:"cache_read"`
	CacheWrite int64 `json:"cache_write"`
	Output     int64 `json:"output"`
	DurationMs int64 `json:"duration_ms"`
}

// ErrNoUsage means the response had no usable usage block.
var ErrNoUsage = errors.New("响应里没有 usage 用量信息")

// Totals are the sums over rounds 2..N (round 1 only writes the cache and is excluded).
type Totals struct {
	Input      int64
	CacheRead  int64
	CacheWrite int64
}

// SumRounds adds up rounds 2..N.
func SumRounds(rounds []Usage) Totals {
	var t Totals
	for i, u := range rounds {
		if i == 0 {
			continue
		}
		t.Input += u.Input
		t.CacheRead += u.CacheRead
		t.CacheWrite += u.CacheWrite
	}
	return t
}

// Rate returns sum(cache read, rounds 2..N) / sum(total input, rounds 2..N), where
// "total input" is Usage.Input (cached parts included, see Usage). It returns nil when
// the input sum is 0 (fewer than 2 rounds, or the upstream reported no input tokens).
func Rate(rounds []Usage) *float64 {
	t := SumRounds(rounds)
	if t.Input <= 0 {
		return nil
	}
	r := float64(t.CacheRead) / float64(t.Input)
	return &r
}

// ParseResponse extracts usage and the answer text from a 2xx response body.
func ParseResponse(protocol string, body []byte) (Usage, string, error) {
	if protocol == ProtocolAnthropic {
		return parseAnthropic(body)
	}
	return parseOpenAI(body)
}

func parseAnthropic(body []byte) (Usage, string, error) {
	var r struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage *struct {
			InputTokens              int64 `json:"input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return Usage{}, "", errors.New("响应不是合法 JSON")
	}
	if r.Usage == nil {
		return Usage{}, "", ErrNoUsage
	}
	var sb strings.Builder
	for _, c := range r.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	u := r.Usage
	return Usage{
		Input:      u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens,
		CacheRead:  u.CacheReadInputTokens,
		CacheWrite: u.CacheCreationInputTokens,
		Output:     u.OutputTokens,
	}, sb.String(), nil
}

func parseOpenAI(body []byte) (Usage, string, error) {
	var r struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens        int64 `json:"prompt_tokens"`
			CompletionTokens    int64 `json:"completion_tokens"`
			PromptTokensDetails *struct {
				CachedTokens int64 `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return Usage{}, "", errors.New("响应不是合法 JSON")
	}
	if r.Usage == nil {
		return Usage{}, "", ErrNoUsage
	}
	var answer string
	if len(r.Choices) > 0 {
		answer = r.Choices[0].Message.Content
	}
	u := Usage{Input: r.Usage.PromptTokens, Output: r.Usage.CompletionTokens}
	if d := r.Usage.PromptTokensDetails; d != nil {
		u.CacheRead = d.CachedTokens
	}
	return u, answer, nil
}
