// Package ports defines the only boundary between the nam core and NewAPI.
//
// The core never imports new-api packages. Two adapters implement these
// interfaces: the embedded adapter (nambridge, compiled into new-api) and the
// standalone adapter (HTTP client against the NewAPI admin API).
package ports

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
)

// Run modes reported by Auth.Mode().
const (
	ModeEmbedded   = "embedded"
	ModeStandalone = "standalone"
)

// NewAPI log types used by nam.
const (
	LogTypeConsume = 2
	LogTypeError   = 5
)

// NewAPI channel status values. 0 is never written (PUT cannot zero non-pointer fields).
const (
	ChannelEnabled        = 1
	ChannelManualDisabled = 2
	ChannelAutoDisabled   = 3
)

// ErrNotFound is returned when a channel or option does not exist.
var ErrNotFound = errors.New("not found")

// Channel is the subset of a NewAPI channel nam reads. NewAPI stays the source of truth;
// nam never persists these fields except as a drift hash.
type Channel struct {
	ID           int      `json:"id"`
	Type         int      `json:"type"`
	Name         string   `json:"name"`
	Key          string   `json:"key,omitempty"` // may be empty when the backend hides it
	BaseURL      string   `json:"base_url"`
	Models       []string `json:"models"`
	Group        string   `json:"group"`
	Priority     int64    `json:"priority"`
	Weight       uint     `json:"weight"`
	Tag          string   `json:"tag"`
	Status       int      `json:"status"`
	TestModel    string   `json:"test_model"`
	ResponseTime int      `json:"response_time"` // ms, last test
	TestTime     int64    `json:"test_time"`
	Balance      float64  `json:"balance"`
	UsedQuota    int64    `json:"used_quota"`
	CreatedTime  int64    `json:"created_time"`
}

// ChannelCreate is what nam sends when creating a channel.
type ChannelCreate struct {
	Type      int      `json:"type"` // nam always uses 58 (PassThrough)
	Name      string   `json:"name"` // must be unique; adapters find the new ID by exact name
	Key       string   `json:"key"`
	BaseURL   string   `json:"base_url"`
	Models    []string `json:"models"`
	Group     string   `json:"group"`
	Priority  int64    `json:"priority"`
	Weight    uint     `json:"weight"`
	Tag       string   `json:"tag"`
	Status    int      `json:"status"`
	TestModel string   `json:"test_model"`
}

// ChannelPatch updates only non-nil fields.
type ChannelPatch struct {
	Name      *string   `json:"name,omitempty"`
	Key       *string   `json:"key,omitempty"`
	BaseURL   *string   `json:"base_url,omitempty"`
	Models    *[]string `json:"models,omitempty"`
	Group     *string   `json:"group,omitempty"`
	Priority  *int64    `json:"priority,omitempty"`
	Weight    *uint     `json:"weight,omitempty"`
	Tag       *string   `json:"tag,omitempty"`
	Status    *int      `json:"status,omitempty"`
	TestModel *string   `json:"test_model,omitempty"`
}

// ChannelTestResult is the outcome of NewAPI's channel test.
type ChannelTestResult struct {
	Success  bool    `json:"success"`
	Message  string  `json:"message"`
	Duration float64 `json:"duration"` // seconds
}

// ChannelStore reads and writes NewAPI channels. Writes must go through NewAPI's full
// flow (validation, abilities rebuild, channel cache refresh).
type ChannelStore interface {
	List(ctx context.Context) ([]Channel, error)
	Get(ctx context.Context, id int) (Channel, error)
	// Create returns the new channel ID.
	Create(ctx context.Context, ch ChannelCreate) (int, error)
	Update(ctx context.Context, id int, patch ChannelPatch) error
	Delete(ctx context.Context, id int) error
	Test(ctx context.Context, id int, model string) (ChannelTestResult, error)
}

// OptionStore reads and writes NewAPI options (GroupRatio, AutoGroups, Price, ...).
// Values are raw strings exactly as NewAPI stores them (JSON options are JSON strings).
type OptionStore interface {
	// Get returns the requested keys; missing keys are absent from the map.
	Get(ctx context.Context, keys ...string) (map[string]string, error)
	// Set writes one option and refreshes NewAPI's runtime caches.
	Set(ctx context.Context, key, value string) error
}

// LogRecord is one row from NewAPI's logs table (only the columns nam needs).
type LogRecord struct {
	ID               int64  `json:"id"`
	CreatedAt        int64  `json:"created_at"`
	Type             int    `json:"type"`
	Content          string `json:"content"`
	TokenName        string `json:"token_name"`
	ModelName        string `json:"model_name"`
	Quota            int    `json:"quota"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	UseTime          int    `json:"use_time"`
	IsStream         bool   `json:"is_stream"`
	ChannelID        int    `json:"channel"`
	Group            string `json:"group"`
	RequestID        string `json:"request_id"`
	Other            string `json:"other"` // raw JSON
}

// LogQuery selects logs strictly after AfterID, oldest first.
type LogQuery struct {
	AfterID int64
	Since   int64 // unix seconds; adapters must always bound by time
	Types   []int
	Limit   int // adapters cap at 500
}

// LogSource reads NewAPI logs incrementally.
type LogSource interface {
	// FetchAfter returns logs with id > q.AfterID ordered by id ascending.
	FetchAfter(ctx context.Context, q LogQuery) ([]LogRecord, error)
	// Recent returns the newest logs of one channel since `since`, id descending.
	Recent(ctx context.Context, channelID int, logType int, since int64, limit int) ([]LogRecord, error)
	// DeleteBefore removes NewAPI logs older than the unix timestamp.
	DeleteBefore(ctx context.Context, before int64) (int64, error)
}

// Identity is the authenticated operator.
type Identity struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Role     int    `json:"role"`
}

// Auth guards /api/nam/*. Only NewAPI root (role 100) may pass.
type Auth interface {
	Mode() string
	// Middleware rejects any request that is not from root.
	Middleware() gin.HandlerFunc
	// SecureMiddleware additionally requires a fresh secure verification
	// (embedded: NewAPI 2FA/Passkey; standalone: may be a no-op).
	SecureMiddleware() gin.HandlerFunc
	// Identity returns the operator after Middleware has run.
	Identity(c *gin.Context) (Identity, bool)
}

// RouteRegistrar is optionally implemented by Auth to expose unauthenticated
// routes such as standalone login. Routes are mounted under /api/nam.
type RouteRegistrar interface {
	RegisterPublicRoutes(g *gin.RouterGroup)
	// RegisterPrivateRoutes mounts routes that require Middleware (e.g. change password).
	RegisterPrivateRoutes(g *gin.RouterGroup)
}

// KV is a small persistent key/value store handed to adapters (backed by nam.db).
type KV interface {
	GetKV(ctx context.Context, key string) (string, bool, error)
	SetKV(ctx context.Context, key, value string) error
}

// KVBinder is optionally implemented by adapters that need persistent state.
type KVBinder interface {
	BindKV(kv KV)
}

// BackendInfo describes the NewAPI instance.
type BackendInfo struct {
	Mode          string  `json:"mode"`
	Version       string  `json:"version"`
	Address       string  `json:"address,omitempty"` // standalone only
	PatchP1       bool    `json:"patch_p1"`          // use_time_ms present
	QuotaPerUnit  float64 `json:"quota_per_unit"`
	LastRequestAt int64   `json:"last_request_at,omitempty"`
	LastError     string  `json:"last_error,omitempty"`
}

// Backend reports NewAPI status.
type Backend interface {
	Info(ctx context.Context) (BackendInfo, error)
}

// ModelFetcher lists the models an upstream offers for a key, reusing NewAPI's own
// "fetch models" logic. It has the same semantics as NewAPI POST /api/channel/fetch_models:
// channelType 24 (Gemini) and Ollama use their native endpoints, every other type
// requests {baseURL}/v1/models; an empty baseURL falls back to the channel type's default.
// Implementations must never log or return the key.
type ModelFetcher interface {
	FetchModels(ctx context.Context, baseURL string, channelType int, key string) ([]string, error)
}

// Ports bundles every adapter implementation the core needs.
type Ports struct {
	Channels ChannelStore
	Options  OptionStore
	Logs     LogSource
	Auth     Auth
	Backend  Backend
	// Models is optional; Validate does not require it.
	Models ModelFetcher
}

// Validate reports missing implementations.
func (p Ports) Validate() error {
	switch {
	case p.Channels == nil:
		return errors.New("ports: Channels is nil")
	case p.Options == nil:
		return errors.New("ports: Options is nil")
	case p.Logs == nil:
		return errors.New("ports: Logs is nil")
	case p.Auth == nil:
		return errors.New("ports: Auth is nil")
	case p.Backend == nil:
		return errors.New("ports: Backend is nil")
	}
	return nil
}
