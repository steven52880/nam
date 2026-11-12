package cachetest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Request is one round to send.
type Request struct {
	ChannelID int
	Protocol  string
	Body      []byte
}

// Result is a parsed successful round.
type Result struct {
	Usage  Usage
	Answer string
}

// Sender sends one round. Tests replace it with a fake.
type Sender interface {
	Send(ctx context.Context, req Request) (Result, error)
}

// ComposeKey builds the channel-pinned key NewAPI understands: sk-<token>-<channelID>.
// A leading "sk-" on the stored token is stripped first.
func ComposeKey(token string, channelID int) string {
	return "sk-" + strings.TrimPrefix(strings.TrimSpace(token), "sk-") + "-" + strconv.Itoa(channelID)
}

// HTTPSender calls NewAPI's /v1 relay. BaseURL has no trailing path (e.g. http://127.0.0.1:3000).
type HTTPSender struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

const maxRespBytes = 4 << 20

// Send posts the body with the channel-pinned key and parses usage.
func (h *HTTPSender) Send(ctx context.Context, req Request) (Result, error) {
	key := ComposeKey(h.Token, req.ChannelID)
	url := strings.TrimRight(h.BaseURL, "/") + Path(req.Protocol)
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(req.Body))
	if err != nil {
		return Result{}, fmt.Errorf("NewAPI 地址无效：%w", err)
	}
	hr.Header.Set("Content-Type", "application/json")
	hr.Header.Set("Authorization", "Bearer "+key)
	if req.Protocol == ProtocolAnthropic {
		hr.Header.Set("x-api-key", key)
		hr.Header.Set("anthropic-version", "2023-06-01")
	}
	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Minute}
	}
	res, err := client.Do(hr)
	if err != nil {
		return Result{}, errors.New("无法连接 NewAPI（" + scrub(err.Error(), h.Token, key) + "），请检查「NewAPI 地址」设置")
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxRespBytes))
	if err != nil {
		return Result{}, errors.New("读取 NewAPI 响应失败：" + scrub(err.Error(), h.Token, key))
	}
	if res.StatusCode/100 != 2 {
		return Result{}, statusError(res.StatusCode, body, h.Token, key)
	}
	u, answer, err := ParseResponse(req.Protocol, body)
	if err != nil {
		return Result{}, err
	}
	return Result{Usage: u, Answer: answer}, nil
}

// statusError turns a non-2xx NewAPI reply into a Chinese error. The token never appears in it.
func statusError(code int, body []byte, secrets ...string) error {
	msg := upstreamMessage(body)
	for _, s := range secrets {
		msg = scrub(msg, s)
	}
	switch code {
	case http.StatusUnauthorized:
		return errors.New("NewAPI 拒绝了测试令牌（401）：令牌无效、已过期或已被禁用。" + suffix(msg))
	case http.StatusForbidden:
		return errors.New("NewAPI 拒绝指定渠道（403）：测试令牌必须属于管理员账号，普通用户令牌不能指定渠道。" + suffix(msg))
	case http.StatusNotFound:
		return errors.New("NewAPI 返回 404：请检查「NewAPI 地址」，或该渠道不存在。" + suffix(msg))
	case http.StatusServiceUnavailable, http.StatusBadGateway:
		return fmt.Errorf("NewAPI 无法使用该渠道（%d）：渠道可能已禁用，或没有可用的该模型。%s", code, suffix(msg))
	}
	return fmt.Errorf("NewAPI 返回错误（%d）。%s", code, suffix(msg))
}

func suffix(msg string) string {
	if msg == "" {
		return ""
	}
	return "详情：" + msg
}

// upstreamMessage extracts error.message (OpenAI/Anthropic style) or falls back to raw text.
func upstreamMessage(body []byte) string {
	var e struct {
		Error   json.RawMessage `json:"error"`
		Message string          `json:"message"`
	}
	text := strings.TrimSpace(string(body))
	if json.Unmarshal(body, &e) == nil {
		var obj struct {
			Message string `json:"message"`
		}
		var s string
		switch {
		case json.Unmarshal(e.Error, &obj) == nil && obj.Message != "":
			text = obj.Message
		case json.Unmarshal(e.Error, &s) == nil && s != "":
			text = s
		case e.Message != "":
			text = e.Message
		}
	}
	r := []rune(text)
	if len(r) > 300 {
		text = string(r[:300]) + "…"
	}
	return text
}

// scrub removes secrets (and their sk- stripped form) from text.
func scrub(text string, secrets ...string) string {
	for _, s := range secrets {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		text = strings.ReplaceAll(text, s, "***")
		if t := strings.TrimPrefix(s, "sk-"); t != s && t != "" {
			text = strings.ReplaceAll(text, t, "***")
		}
	}
	return text
}
