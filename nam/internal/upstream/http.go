package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	defaultTimeout = 20 * time.Second
	maxBodyBytes   = 8 << 20
)

var defaultHTTPClient = &http.Client{Timeout: defaultTimeout}

// client returns the shared client of the site, or a 20s-timeout default.
func (s Site) client() *http.Client {
	if s.HTTP != nil {
		return s.HTTP
	}
	return defaultHTTPClient
}

func joinURL(base, path string) string {
	return strings.TrimRight(base, "/") + path
}

// apiError is a business-level failure reported by the upstream (success=false etc).
type apiError struct{ msg string }

func (e *apiError) Error() string { return e.msg }

// scrubURLError drops the query string (which may carry a key) from transport errors.
func scrubURLError(err error) error {
	var ue *url.Error
	if !errors.As(err, &ue) {
		return err
	}
	where := ue.URL
	if u, perr := url.Parse(ue.URL); perr == nil {
		where = u.Host + u.Path
	}
	return fmt.Errorf("%s %s: %w", ue.Op, where, ue.Err)
}

// doRaw performs one request and returns the status and (size-capped) body.
func doRaw(ctx context.Context, hc *http.Client, method, rawURL string, headers map[string]string, body any) (int, []byte, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("encode request: %w", err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, rd)
	if err != nil {
		return 0, nil, scrubURLError(err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return 0, nil, scrubURLError(err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return 0, nil, scrubURLError(err)
	}
	if len(data) > maxBodyBytes {
		return 0, nil, fmt.Errorf("response body exceeds %d bytes", maxBodyBytes)
	}
	return resp.StatusCode, data, nil
}

// errMessage extracts {message} or {error:{message}} from an error body.
func errMessage(data []byte) string {
	var m struct {
		Message string `json:"message"`
		Error   *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &m) != nil {
		return ""
	}
	if m.Error != nil && m.Error.Message != "" {
		return m.Error.Message
	}
	return m.Message
}

// doJSON performs a request and decodes a 2xx JSON body into out (if non-nil).
// HTTP 401/403 map to ErrAuth.
func doJSON(ctx context.Context, hc *http.Client, method, rawURL string, headers map[string]string, body, out any) error {
	status, data, err := doRaw(ctx, hc, method, rawURL, headers, body)
	if err != nil {
		return err
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return fmt.Errorf("HTTP %d: %w", status, ErrAuth)
	}
	if status < 200 || status >= 300 {
		if m := errMessage(data); m != "" {
			return fmt.Errorf("HTTP %d: %s", status, m)
		}
		return fmt.Errorf("HTTP %d", status)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func getJSON(ctx context.Context, hc *http.Client, rawURL string, headers map[string]string, out any) error {
	return doJSON(ctx, hc, http.MethodGet, rawURL, headers, nil, out)
}

func postJSON(ctx context.Context, hc *http.Client, rawURL string, headers map[string]string, body, out any) error {
	return doJSON(ctx, hc, http.MethodPost, rawURL, headers, body, out)
}

// envelopeHead is the {success,message} part of NewAPI responses.
type envelopeHead struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func isAuthMessage(msg string) bool {
	l := strings.ToLower(msg)
	return strings.Contains(msg, "未登录") || strings.Contains(msg, "无权") || strings.Contains(l, "access token")
}

func (e envelopeHead) check() error {
	if e.Success {
		return nil
	}
	if isAuthMessage(e.Message) {
		return fmt.Errorf("%s: %w", e.Message, ErrAuth)
	}
	if e.Message == "" {
		return &apiError{msg: "upstream returned success=false"}
	}
	return &apiError{msg: e.Message}
}

// callEnvelope decodes {success,message,data}; data is unmarshalled into out.
func callEnvelope(ctx context.Context, hc *http.Client, method, rawURL string, headers map[string]string, body, out any) error {
	var env struct {
		envelopeHead
		Data json.RawMessage `json:"data"`
	}
	if err := doJSON(ctx, hc, method, rawURL, headers, body, &env); err != nil {
		return err
	}
	if err := env.check(); err != nil {
		return err
	}
	if out == nil || len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("decode data: %w", err)
	}
	return nil
}

func sortedUnique(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// listModelsOpenAI is GET {base}/v1/models with a Bearer key.
func listModelsOpenAI(ctx context.Context, s Site, key string) ([]string, error) {
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	hdr := map[string]string{"Authorization": "Bearer " + key}
	if err := getJSON(ctx, s.client(), joinURL(s.BaseURL, "/v1/models"), hdr, &out); err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	ids := make([]string, 0, len(out.Data))
	for _, m := range out.Data {
		ids = append(ids, m.ID)
	}
	return sortedUnique(ids), nil
}

// singleKeyToken is the one pseudo-token of key-only upstreams.
func singleKeyToken(s Site) (Token, error) {
	if s.Cred.Key == "" {
		return Token{}, fmt.Errorf("no key configured: %w", ErrUnsupported)
	}
	return Token{Name: "(sk-key)", Key: s.Cred.Key, Status: 1, Unlimited: false}, nil
}
