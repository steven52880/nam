package cachetest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/steven52880/nam/internal/crypto"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
)

// ErrNotDirectable means the channel has no upstream/token that nam can call directly.
var ErrNotDirectable = resp.Invalid("该渠道不是由 nam 创建或接管的，无法直连上游测试")

// DirectSender calls the upstream site itself (not NewAPI) with the upstream token.
// BaseURL is the upstream address; a trailing slash or "/v1" is tolerated.
type DirectSender struct {
	BaseURL string
	Key     string
	Client  *http.Client
}

// directBase trims a trailing slash and a trailing /v1 so Path() does not double it.
func directBase(raw string) string {
	b := strings.TrimRight(strings.TrimSpace(raw), "/")
	b = strings.TrimSuffix(b, "/v1")
	return strings.TrimRight(b, "/")
}

// Send posts the body to the upstream and parses usage.
func (d *DirectSender) Send(ctx context.Context, req Request) (Result, error) {
	key := strings.TrimSpace(d.Key)
	url := directBase(d.BaseURL) + Path(req.Protocol)
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(req.Body))
	if err != nil {
		return Result{}, errors.New("上游地址无效：" + scrub(err.Error(), key))
	}
	hr.Header.Set("Content-Type", "application/json")
	hr.Header.Set("Authorization", "Bearer "+key)
	if req.Protocol == ProtocolAnthropic {
		hr.Header.Set("x-api-key", key)
		hr.Header.Set("anthropic-version", "2023-06-01")
	}
	client := d.Client
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Minute}
	}
	res, err := client.Do(hr)
	if err != nil {
		return Result{}, fmt.Errorf("连不上上游 %s", hr.URL.Host)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxRespBytes))
	if err != nil {
		return Result{}, errors.New("读取上游响应失败：" + scrub(err.Error(), key))
	}
	if res.StatusCode/100 != 2 {
		return Result{}, directStatusError(res.StatusCode, body, key)
	}
	u, answer, err := ParseResponse(req.Protocol, body)
	if err != nil {
		return Result{}, err
	}
	return Result{Usage: u, Answer: answer}, nil
}

// directStatusError turns a non-2xx upstream reply into a Chinese error without the key.
func directStatusError(code int, body []byte, secrets ...string) error {
	msg := upstreamMessage(body)
	for _, s := range secrets {
		msg = scrub(msg, s)
	}
	if code == http.StatusUnauthorized || code == http.StatusForbidden {
		return fmt.Errorf("上游拒绝了令牌（HTTP %d）。%s", code, suffix(msg))
	}
	return fmt.Errorf("上游返回错误（HTTP %d）。%s", code, suffix(msg))
}

// ResolveDirect finds the upstream base address and the decrypted token key of a channel.
// It returns a 400 when the channel was not created/adopted by nam.
func ResolveDirect(ctx context.Context, st *store.Store, box *crypto.Box, channelID int) (baseURL, key string, err error) {
	var ext store.ChannelExt
	if err := st.DB.WithContext(ctx).Where("channel_id = ?", channelID).Limit(1).Find(&ext).Error; err != nil {
		return "", "", err
	}
	if ext.ChannelID == 0 || ext.UpstreamID == nil || ext.TokenID == nil {
		return "", "", ErrNotDirectable
	}
	up, err := st.GetUpstream(ctx, *ext.UpstreamID)
	if errors.Is(err, store.ErrNotFound) {
		return "", "", ErrNotDirectable
	}
	if err != nil {
		return "", "", err
	}
	var tok store.UpstreamToken
	if err := st.DB.WithContext(ctx).Where("id = ?", *ext.TokenID).Limit(1).Find(&tok).Error; err != nil {
		return "", "", err
	}
	if tok.ID == 0 || tok.UpstreamID != up.ID {
		return "", "", ErrNotDirectable
	}
	if len(tok.Key) == 0 {
		return "", "", resp.Invalid("该渠道使用的上游令牌没有保存密钥，无法直连上游测试")
	}
	pt, err := box.Open(tok.Key)
	if err != nil {
		return "", "", errors.New("上游令牌无法解密（NAM_SECRET 是否变了？）")
	}
	key = strings.TrimSpace(string(pt))
	if key == "" {
		return "", "", resp.Invalid("该渠道使用的上游令牌密钥为空，无法直连上游测试")
	}
	if strings.TrimSpace(up.BaseURL) == "" {
		return "", "", resp.Invalid("上游没有配置地址，无法直连测试")
	}
	return up.BaseURL, key, nil
}
