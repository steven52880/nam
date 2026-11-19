package store

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// Upstream types.
const (
	UpstreamNewAPI    = "newapi"
	UpstreamSKKey     = "sk-key"
	UpstreamSub2API   = "sub2api"
	UpstreamOneHub    = "onehub"
	UpstreamOfficial  = "official"
	UpstreamGeneric   = "generic"
	UpstreamRightCode = "rightcode"
)

// Credential types.
const (
	CredAccessToken    = "access_token"
	CredPassword       = "password"
	CredSKKey          = "sk_key"
	CredCookie         = "cookie"          // NewAPI / OneHub browser session cookie
	CredSub2APISession = "sub2api_session" // sub2api access+refresh token pair, auto-refreshed
)

// Upstream is one upstream site.
type Upstream struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Name     string `gorm:"uniqueIndex;size:64;not null" json:"name"`
	Type     string `gorm:"size:16;not null" json:"type"`
	BaseURL  string `gorm:"size:255;not null" json:"base_url"`
	CredType string `gorm:"size:16" json:"cred_type"`
	// Cred is AES-GCM sealed JSON (see internal/crypto); never serialized.
	Cred []byte `json:"-"`
	// CredUpdatedAt is when the credential last changed (Unix seconds): API write or token rotation.
	CredUpdatedAt  int64   `json:"cred_updated_at"`
	PlatformUserID int     `json:"platform_user_id"`
	CurrencyRate   float64 `gorm:"not null;default:1" json:"currency_rate"` // 上游货币汇率：元 / 上游 1 美元额度
	Tags           string  `gorm:"size:255" json:"tags"`                    // comma separated
	Remark         string  `json:"remark"`
	AdapterConfig  string  `json:"adapter_config"` // JSON, generic adapter only

	BalanceUSD      *float64 `json:"balance_usd"`
	BalanceAt       int64    `json:"balance_at"`
	LastSyncAt      int64    `json:"last_sync_at"`
	LastSyncError   string   `json:"last_sync_error"`
	ConsecutiveFail int      `json:"consecutive_fail"`
	// SyncParts is the latest outcome of each synced part (balance/tokens/groups/model_prices).
	// Read-only for gorm: written only through an atomic json_set UPDATE in the upstream syncer,
	// so that SaveUpstream never overwrites it with a stale copy.
	SyncParts SyncParts `gorm:"type:text;<-:false" json:"sync_parts"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Sync part names and states used in Upstream.SyncParts.
const (
	SyncPartBalance     = "balance"
	SyncPartTokens      = "tokens"
	SyncPartGroups      = "groups"
	SyncPartModelPrices = "model_prices"

	SyncOK          = "ok"          // synced
	SyncFail        = "fail"        // pulling failed
	SyncUnsupported = "unsupported" // this upstream type does not provide it
	SyncEmpty       = "empty"       // synced but the upstream returned nothing (model_prices only)
)

// SyncPart is the latest sync outcome of one part of an upstream.
type SyncPart struct {
	State   string `json:"state"`
	At      int64  `json:"at"`
	Message string `json:"message"`
	Count   *int   `json:"count"` // tokens / groups / model prices; nil for balance
}

// SyncParts maps a part name to its latest outcome. A nil map serializes as {}.
type SyncParts map[string]SyncPart

// Scan reads the JSON column. NULL, empty or malformed values yield an empty map so that a
// bad legacy value can never make an upstream unreadable.
func (p *SyncParts) Scan(src any) error {
	var raw []byte
	switch v := src.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	}
	m := map[string]SyncPart{}
	if len(raw) == 0 || json.Unmarshal(raw, &m) != nil || m == nil {
		m = map[string]SyncPart{}
	}
	*p = m
	return nil
}

// Value implements driver.Valuer.
func (p SyncParts) Value() (driver.Value, error) {
	b, err := p.MarshalJSON()
	return string(b), err
}

// MarshalJSON makes a nil map render as {} instead of null.
func (p SyncParts) MarshalJSON() ([]byte, error) {
	if p == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(map[string]SyncPart(p))
}

// UpstreamGroup is a group ratio synced from an upstream.
type UpstreamGroup struct {
	ID         uint    `gorm:"primaryKey" json:"id"`
	UpstreamID uint    `gorm:"uniqueIndex:ux_upgroup;not null" json:"upstream_id"`
	Name       string  `gorm:"uniqueIndex:ux_upgroup;size:64;not null" json:"name"`
	Ratio      float64 `json:"ratio"`
	Desc       string  `json:"desc"`
	UpdatedAt  int64   `json:"updated_at"`
}

// UpstreamToken is a token on an upstream (synced or manual).
type UpstreamToken struct {
	ID         uint     `gorm:"primaryKey" json:"id"`
	UpstreamID uint     `gorm:"index;not null" json:"upstream_id"`
	RemoteID   string   `gorm:"size:64" json:"remote_id"` // id on upstream, empty for manual
	Name       string   `gorm:"size:128" json:"name"`
	Group      string   `gorm:"size:64" json:"group"`
	Key        []byte   `json:"-"`                       // sealed
	KeyHint    string   `gorm:"size:32" json:"key_hint"` // e.g. sk-ab…yz
	QuotaUSD   *float64 `json:"quota_usd"`
	Unlimited  bool     `json:"unlimited"`
	Status     int      `json:"status"`
	Manual     bool     `json:"manual"`
	UpdatedAt  int64    `json:"updated_at"`
}

// UpstreamModelPrice is per-model upstream pricing returned by adapter Pricing.
type UpstreamModelPrice struct {
	ID         uint     `gorm:"primaryKey" json:"id"`
	UpstreamID uint     `gorm:"uniqueIndex:ux_upprice;not null" json:"upstream_id"`
	Group      string   `gorm:"uniqueIndex:ux_upprice;size:64" json:"group"`
	Model      string   `gorm:"uniqueIndex:ux_upprice;size:128;not null" json:"model"`
	ModelRatio *float64 `json:"model_ratio"`
	Input      *float64 `json:"input"` // USD / 1M in upstream credit
	Output     *float64 `json:"output"`
	CacheRead  *float64 `json:"cache_read"`
	CacheWrite *float64 `json:"cache_write"`
	PerRequest *float64 `json:"per_request"`
	UpdatedAt  int64    `json:"updated_at"`
}

// ChannelExt holds nam-only data for a NewAPI channel. Channel fields themselves live in NewAPI.
// The channel's vendor and its discount are not stored: the vendor follows the channel's
// groups, the discount is computed from cached upstream prices unless ManualDiscount is set.
type ChannelExt struct {
	ChannelID      int      `gorm:"primaryKey;autoIncrement:false" json:"channel_id"`
	UpstreamID     *uint    `gorm:"index" json:"upstream_id"`
	TokenID        *uint    `json:"token_id"`
	ManualDiscount *float64 `json:"manual_discount"` // 上游分组倍率手填值；NULL = 现算
	CacheOverride  string   `json:"cache_override"`  // JSON: {"read":0.1,"write":1.25}
	ModelDiscount  string   `json:"model_discount"`  // JSON: {"model":0.35}
	UpdatedAt      int64    `json:"updated_at"`
}

// Draft ops.
const (
	DraftCreate = "create"
	DraftUpdate = "update"
	DraftDelete = "delete"
)

// ChannelDraft is a pending change. ChannelID < 0 means a new channel (temporary ID).
type ChannelDraft struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	ChannelID int    `gorm:"uniqueIndex;not null" json:"channel_id"`
	Op        string `gorm:"size:8;not null" json:"op"`
	Diff      string `gorm:"not null" json:"diff"` // JSON field -> new value (ports.ChannelCreate/Patch + ext fields)
	LastError string `json:"last_error"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// Vendor is one vendor = one NewAPI group.
type Vendor struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Name      string `gorm:"uniqueIndex;size:64;not null" json:"name"`
	GroupName string `gorm:"uniqueIndex;size:64;not null" json:"group_name"`
	Models    string `json:"models"` // JSON array of exact model names
	// Prefixes are case-insensitive model-name prefixes; the longest match wins and
	// explicit Models take priority over prefixes.
	Prefixes []string `gorm:"serializer:json" json:"prefixes"`
	Order    int      `json:"order"` // position in AutoGroups
	// ChannelType is the NewAPI channel type nam uses for this vendor's channels.
	// 58 = PassThrough (OpenAI/Claude formats). Gemini vendors use 24 (native Gemini):
	// M0 ① showed PassThrough does not bill Gemini-native usage correctly.
	ChannelType int `gorm:"not null;default:58" json:"channel_type"`
}

// NewAPI channel types nam creates.
const (
	ChannelTypePassThrough = 58
	ChannelTypeGemini      = 24
)

// Pricing modes and states.
const (
	PricingFixed  = "fixed"
	PricingFollow = "follow"

	PricingSynced    = "synced"     // 已同步
	PricingToWrite   = "to_write"   // 待写入
	PricingPending   = "pending"    // 待确认（变化超过保护阈值）
	PricingNoChannel = "no_channel" // 无可用渠道
	PricingUnset     = "unset"      // 未设置
)

// VendorPricing is a vendor's pricing settings. It stores only what NewAPI does not have.
//
// Fixed mode: the price is NewAPI's GroupRatio; DraftRatio is a change not written yet.
// Follow mode: the price is 参考成本 × Markup. Prices are multiples of the official price
// (0.30 = 30%); the group ratio is price × 美元参考汇率 ÷ 站点汇率.
type VendorPricing struct {
	VendorID   uint     `gorm:"primaryKey;autoIncrement:false" json:"vendor_id"`
	Mode       string   `gorm:"size:8;default:fixed" json:"mode"`
	Markup     *float64 `json:"markup"`      // 加价比例（跟随模式）：售价 = 参考成本 × 加价比例
	DraftRatio *float64 `json:"draft_ratio"` // 固定模式待写入 NewAPI 的倍率草稿；没有草稿为 NULL
	UpdatedAt  int64    `json:"updated_at"`
}

// OfficialPrice is the official price book. USD per 1M tokens, or USD per request.
type OfficialPrice struct {
	Model        string   `gorm:"primaryKey;size:128" json:"model"`
	Vendor       string   `gorm:"size:64" json:"vendor"`
	Input        *float64 `json:"input"`
	Output       *float64 `json:"output"`
	CacheRead    *float64 `json:"cache_read"`
	CacheWrite5m *float64 `json:"cache_write_5m"`
	CacheWrite1h *float64 `json:"cache_write_1h"`
	PerRequest   *float64 `json:"per_request"`
	Source       string   `gorm:"size:32" json:"source"`
	UpdatedAt    int64    `json:"updated_at"`
}

// PriceOverride replaces an official price manually.
type PriceOverride struct {
	Model         string   `gorm:"primaryKey;size:128" json:"model"`
	PerRequest    bool     `json:"per_request_mode"`
	Input         *float64 `json:"input"`
	Output        *float64 `json:"output"`
	CacheRead     *float64 `json:"cache_read"`
	CacheWrite5m  *float64 `json:"cache_write_5m"`
	CacheWrite1h  *float64 `json:"cache_write_1h"`
	PerRequestUSD *float64 `json:"per_request"`
	Remark        string   `json:"remark"`
	UpdatedAt     int64    `json:"updated_at"`
}

// ModelAlias maps an upstream model name to an official one.
type ModelAlias struct {
	Alias    string `gorm:"primaryKey;size:128" json:"alias"`
	Official string `gorm:"size:128;index;not null" json:"official"`
}

// Audit kinds, triggers and results.
const (
	AuditChannel  = "channel"
	AuditPrice    = "price"
	AuditSetting  = "setting"
	AuditUpstream = "upstream"

	TriggerAuto   = "auto"
	TriggerManual = "manual"

	ResultOK      = "ok"
	ResultFail    = "fail"
	ResultSkipped = "skipped"
)

// AuditLog records every write to NewAPI (and important nam writes) with the prior value.
type AuditLog struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	At         int64  `gorm:"index" json:"at"`
	Kind       string `gorm:"size:16;index" json:"kind"`
	Target     string `gorm:"size:128" json:"target"` // e.g. "channel:12", "option:GroupRatio"
	Action     string `gorm:"size:32" json:"action"`  // create/update/delete/set/rollback/http
	Summary    string `json:"summary"`
	Before     string `json:"before"`
	After      string `json:"after"`
	Request    string `json:"request"`
	Response   string `json:"response"`
	Trigger    string `gorm:"size:8" json:"trigger"`
	Result     string `gorm:"size:8;index" json:"result"`
	Error      string `json:"error"`
	Operator   string `gorm:"size:64" json:"operator"`
	RollbackOf *uint  `json:"rollback_of"`
}

// PriceHistory records every pricing write so it can be rolled back.
//
// Vendor pricing rows use Key "vendor:<id>" and VendorID; Before/After are JSON
// {"mode","sell_price","group_ratio"}. Reason is manual / auto / confirm / force.
type PriceHistory struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	At         int64  `gorm:"index" json:"at"`
	Key        string `gorm:"size:64" json:"key"`
	VendorID   *uint  `gorm:"index" json:"vendor_id"`
	Before     string `json:"before"`
	After      string `json:"after"`
	Reason     string `json:"reason"`
	AuditID    uint   `json:"audit_id"`
	RolledBack bool   `json:"rolled_back"`
}

// NotifyChannel is a notification transport.
type NotifyChannel struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	Kind    string `gorm:"uniqueIndex;size:16" json:"kind"` // telegram/webhook/email
	Enabled bool   `json:"enabled"`
	Config  []byte `json:"-"` // sealed JSON (contains tokens/passwords)
}

// NotifyRule configures one event type.
type NotifyRule struct {
	ID         uint    `gorm:"primaryKey" json:"id"`
	Event      string  `gorm:"size:32;index" json:"event"`
	UpstreamID *uint   `json:"upstream_id"` // per-upstream override (low balance)
	Threshold  float64 `json:"threshold"`
	Channels   string  `json:"channels"` // comma separated kinds
	CooldownS  int     `json:"cooldown_s"`
	Enabled    bool    `json:"enabled"`
}

// NotifyEvent is one fired event (dedupe by Key within cooldown).
type NotifyEvent struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	At       int64  `gorm:"index" json:"at"`
	Event    string `gorm:"size:32" json:"event"`
	Key      string `gorm:"size:128;index" json:"key"`
	Content  string `json:"content"`
	Delivery string `json:"delivery"`              // JSON kind -> "ok"/error
	Status   string `gorm:"size:16" json:"status"` // sent/deduped/partial
}

// Snapshot stores hashes for change detection (announcements, prices).
type Snapshot struct {
	Key       string `gorm:"primaryKey;size:128" json:"key"`
	Hash      string `gorm:"size:64" json:"hash"`
	Content   string `json:"content"`
	UpdatedAt int64  `json:"updated_at"`
}

// Setting is a nam key/value setting (also backs ports.KV).
type Setting struct {
	Key   string `gorm:"primaryKey;size:64" json:"key"`
	Value string `json:"value"`
}

// JobRun records scheduler executions.
type JobRun struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Job      string `gorm:"size:32;index" json:"job"`
	StartAt  int64  `gorm:"index" json:"start_at"`
	Duration int64  `json:"duration_ms"`
	Result   string `gorm:"size:8" json:"result"`
	Message  string `json:"message"`
}

// AllModels is used for AutoMigrate.
func AllModels() []any {
	return []any{
		&Upstream{}, &UpstreamGroup{}, &UpstreamToken{}, &UpstreamModelPrice{},
		&ChannelExt{}, &ChannelDraft{}, &Vendor{}, &VendorPricing{},
		&OfficialPrice{}, &PriceOverride{}, &ModelAlias{},
		&StatsHourly{}, &LogCursor{}, &AuditLog{}, &PriceHistory{},
		&NotifyChannel{}, &NotifyRule{}, &NotifyEvent{}, &Snapshot{}, &Setting{}, &JobRun{},
		&CacheTest{},
	}
}
