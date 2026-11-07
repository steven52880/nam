package standalone

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/steven52880/nam/internal/httpx"
	"github.com/steven52880/nam/ports"
)

const (
	minRequestGap  = 200 * time.Millisecond
	requestTimeout = 60 * time.Second
	rootRole       = 100
)

// APIError is a NewAPI envelope with success=false (or a non-2xx reply carrying a message).
type APIError struct {
	Message string
	Status  int // 0 when the HTTP status was 200
}

func (e *APIError) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("newapi: %s (http %d)", e.Message, e.Status)
	}
	return "newapi: " + e.Message
}

type envelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// Client talks to NewAPI with the root access token. Requests are serialized with a
// minimum gap (httpx) so nam never hammers the gateway.
type Client struct {
	base   string
	userID int
	hc     *httpx.Client

	mu        sync.Mutex
	lastAt    int64
	lastError string
}

// NewClient builds a client with the production 200ms request gap.
func NewClient(cfg Config) *Client { return newClient(cfg, minRequestGap) }

func newClient(cfg Config, gap time.Duration) *Client {
	hc := httpx.New(requestTimeout, gap)
	// NewAPI's authHelper accepts the raw access token (a "Bearer " prefix is optional)
	// and requires New-Api-User to equal the token owner's id.
	hc.Header.Set("Authorization", cfg.Token)
	hc.Header.Set("New-Api-User", strconv.Itoa(cfg.UserID))
	return &Client{base: strings.TrimRight(cfg.NewAPIURL, "/"), userID: cfg.UserID, hc: hc}
}

func (c *Client) record(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastAt = time.Now().Unix()
	if err != nil {
		c.lastError = err.Error()
		return
	}
	c.lastError = ""
}

func (c *Client) lastState() (int64, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastAt, c.lastError
}

// do sends one request and decodes the whole response body into out (no envelope handling).
// A non-2xx reply that still carries a NewAPI envelope becomes an *APIError.
func (c *Client) do(ctx context.Context, method, path string, q url.Values, body, out any) error {
	u := c.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	raw, err := c.hc.Do(ctx, method, u, body, out, nil)
	if err != nil {
		var se *httpx.StatusError
		if errors.As(err, &se) {
			var env envelope
			if json.Unmarshal(raw, &env) == nil && env.Message != "" {
				err = &APIError{Message: env.Message, Status: se.Status}
			}
		}
	}
	c.record(err)
	return err
}

// call sends a request, requires success=true in the envelope and decodes data into out.
func (c *Client) call(ctx context.Context, method, path string, q url.Values, body, out any) error {
	var env envelope
	if err := c.do(ctx, method, path, q, body, &env); err != nil {
		return err
	}
	if !env.Success {
		err := &APIError{Message: env.Message}
		c.record(err)
		return err
	}
	if out == nil || len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		err = fmt.Errorf("decode %s %s data: %w", method, path, err)
		c.record(err)
		return err
	}
	return nil
}

func isNotFound(err error) bool {
	var ae *APIError
	if !errors.As(err, &ae) {
		return false
	}
	m := strings.ToLower(ae.Message)
	return strings.Contains(m, "record not found") || strings.Contains(m, "not found") || strings.Contains(m, "不存在")
}

func notFound(what string, id int, err error) error {
	return fmt.Errorf("%s %d: %w (%v)", what, id, ports.ErrNotFound, err)
}

// VerifyRoot checks that the configured token belongs to the configured root user.
func (c *Client) VerifyRoot(ctx context.Context) error {
	var self struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		Role     int    `json:"role"`
	}
	if err := c.call(ctx, http.MethodGet, "/api/user/self", nil, nil, &self); err != nil {
		return fmt.Errorf("verify root: %w", err)
	}
	if self.Role != rootRole {
		return fmt.Errorf("verify root: user %q has role %d, root (%d) required", self.Username, self.Role, rootRole)
	}
	if self.ID != c.userID {
		return fmt.Errorf("verify root: token belongs to user %d, NAM_NEWAPI_USER_ID is %d", self.ID, c.userID)
	}
	return nil
}
