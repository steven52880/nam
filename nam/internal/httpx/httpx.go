// Package httpx provides a rate-limited JSON HTTP client shared by the standalone
// adapter and upstream adapters.
package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Client serializes requests with a minimum gap between them.
type Client struct {
	HTTP     *http.Client
	MinGap   time.Duration
	Header   http.Header // default headers added to every request
	mu       sync.Mutex
	lastSend time.Time
	// OnRequest, if set, is called after each request (for metrics / last-request time).
	OnRequest func(method, url string, status int, err error)
}

// New returns a client with the given timeout and minimum gap.
func New(timeout, minGap time.Duration) *Client {
	return &Client{HTTP: &http.Client{Timeout: timeout}, MinGap: minGap, Header: http.Header{}}
}

func (c *Client) wait(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.MinGap > 0 && !c.lastSend.IsZero() {
		d := c.MinGap - time.Since(c.lastSend)
		if d > 0 {
			t := time.NewTimer(d)
			select {
			case <-ctx.Done():
				t.Stop()
				return ctx.Err()
			case <-t.C:
			}
		}
	}
	c.lastSend = time.Now()
	return nil
}

// StatusError is returned for non-2xx responses.
type StatusError struct {
	Status int
	Body   string
}

func (e *StatusError) Error() string {
	b := e.Body
	if len(b) > 300 {
		b = b[:300] + "…"
	}
	return fmt.Sprintf("http %d: %s", e.Status, b)
}

// Do sends a request; body (if non-nil) is JSON-encoded; out (if non-nil) is JSON-decoded.
// Returns the raw response body for auditing.
func (c *Client) Do(ctx context.Context, method, url string, body any, out any, hdr http.Header) ([]byte, error) {
	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rd)
	if err != nil {
		return nil, err
	}
	for k, vs := range c.Header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	for k, vs := range hdr {
		req.Header.Del(k)
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if c.OnRequest != nil {
			c.OnRequest(method, url, 0, err)
		}
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if c.OnRequest != nil {
		c.OnRequest(method, url, resp.StatusCode, err)
	}
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return raw, &StatusError{Status: resp.StatusCode, Body: string(raw)}
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return raw, fmt.Errorf("decode %s %s: %w", method, url, err)
		}
	}
	return raw, nil
}
