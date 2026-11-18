// Package channels implements nam's channel layer on top of NewAPI: per-channel extension
// data, drafts, apply, batch creation and drift detection.
//
// NewAPI stays the source of truth for channel fields. nam stores only channel_ext
// (upstream, token, vendor, discount) plus a snapshot of the managed fields at last apply.
package channels

import "github.com/steven52880/nam/internal/store"

// Fields are NewAPI channel fields a draft may change. Nil = unchanged.
// Key is never stored in drafts; it is resolved from the token at apply time.
type Fields struct {
	Name      *string   `json:"name,omitempty"`
	BaseURL   *string   `json:"base_url,omitempty"`
	Models    *[]string `json:"models,omitempty"`
	Group     *string   `json:"group,omitempty"`
	Priority  *int64    `json:"priority,omitempty"`
	Weight    *uint     `json:"weight,omitempty"`
	Tag       *string   `json:"tag,omitempty"`
	Status    *int      `json:"status,omitempty"`
	TestModel *string   `json:"test_model,omitempty"`
	Type      *int      `json:"type,omitempty"` // create only
}

func (f Fields) empty() bool {
	return f.Name == nil && f.BaseURL == nil && f.Models == nil && f.Group == nil && f.Priority == nil &&
		f.Weight == nil && f.Tag == nil && f.Status == nil && f.TestModel == nil && f.Type == nil
}

// ExtFields are nam-only fields (never written to NewAPI).
type ExtFields struct {
	UpstreamID     *uint    `json:"upstream_id,omitempty"`
	TokenID        *uint    `json:"token_id,omitempty"`
	Vendor         *string  `json:"vendor,omitempty"`
	Discount       *float64 `json:"discount,omitempty"`
	DiscountSource *string  `json:"discount_source,omitempty"`
}

func (e ExtFields) empty() bool {
	return e.UpstreamID == nil && e.TokenID == nil && e.Vendor == nil && e.Discount == nil && e.DiscountSource == nil
}

// Change is one edit submitted by the UI. ChannelID > 0 is an existing NewAPI channel,
// ChannelID < 0 is a not-yet-created draft.
type Change struct {
	ChannelID int       `json:"channel_id"`
	Fields    Fields    `json:"fields"`
	Ext       ExtFields `json:"ext"`
}

// draftBody is what channel_draft.diff stores.
type draftBody struct {
	Fields Fields    `json:"fields"`
	Ext    ExtFields `json:"ext"`
	// GroupDrop lists groups to remove from NewAPI at apply time even though they are no
	// longer managed (set only by a vendor group rename).
	GroupDrop []string `json:"group_drop,omitempty"`
}

// VendorUpdate is the result of editing a vendor: the vendor plus how many drafts the
// edit created or changed.
type VendorUpdate struct {
	store.Vendor
	GroupDrafts   int `json:"group_drafts"`
	RenamedDrafts int `json:"renamed_drafts"`
}

// Sync states shown in the channel table.
const (
	StateSynced      = "synced"
	StateDraft       = "draft"
	StateDraftCreate = "draft_create"
	StateDraftDelete = "draft_delete"
	StateDrift       = "drift"
	StateError       = "error"
)

// Row is one channel as the UI sees it: NewAPI values overlaid with pending draft values.
type Row struct {
	ID           int      `json:"id"`
	Type         int      `json:"type"`
	Name         string   `json:"name"`
	Status       int      `json:"status"`
	BaseURL      string   `json:"base_url"`
	Models       []string `json:"models"`
	Group        string   `json:"group"`
	Priority     int64    `json:"priority"`
	Weight       uint     `json:"weight"`
	Tag          string   `json:"tag"`
	TestModel    string   `json:"test_model"`
	ResponseTime int      `json:"response_time"`
	TestTime     int64    `json:"test_time"`
	UsedQuota    int64    `json:"used_quota"`

	Managed        bool     `json:"managed"`
	UpstreamID     *uint    `json:"upstream_id"`
	UpstreamName   string   `json:"upstream_name"`
	TokenID        *uint    `json:"token_id"`
	TokenName      string   `json:"token_name"`
	Vendor         string   `json:"vendor"`
	Discount       float64  `json:"discount"`        // 上游分组折扣
	DiscountSource string   `json:"discount_source"` // auto / manual
	CurrencyRate   *float64 `json:"currency_rate"`   // 上游货币汇率
	ChannelCost    *float64 `json:"channel_cost"`    // 上游渠道售价 = 分组倍率 × 上游货币汇率 ÷ 美元参考汇率，原厂价的倍数，无单位
	BalanceUSD     *float64 `json:"balance_usd"`     // 上游余额（美元）
	BalanceCNY     *float64 `json:"balance_cny"`     // 折合人民币
	UnitCost       *float64 `json:"unit_cost"`       // 单次成本（元）：标准请求 1000 输入（200 缓存）+ 500 输出
	UnitCostModel  string   `json:"unit_cost_model"` // 计算单次成本所用模型

	SyncState   string   `json:"sync_state"`
	DirtyFields []string `json:"dirty_fields"`
	DriftFields []string `json:"drift_fields"`
	DraftID     uint     `json:"draft_id,omitempty"`
	DraftOp     string   `json:"draft_op,omitempty"`
	DraftError  string   `json:"draft_error,omitempty"`
}

// FieldChange is one field in an apply preview.
type FieldChange struct {
	Field  string `json:"field"`
	Old    any    `json:"old"`
	New    any    `json:"new"`
	NewAPI bool   `json:"newapi"` // false = "不写 NewAPI"
}

// Request describes the NewAPI call an apply step will make (mode-agnostic).
type Request struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Body   any    `json:"body,omitempty"`
}

// PlanItem is one draft in the apply preview.
type PlanItem struct {
	DraftID   uint          `json:"draft_id"`
	ChannelID int           `json:"channel_id"`
	Op        string        `json:"op"`
	Name      string        `json:"name"`
	Changes   []FieldChange `json:"changes"`
	Request   *Request      `json:"request,omitempty"` // nil when only nam fields change
}

// Result is the outcome of applying one draft.
type Result struct {
	DraftID   uint   `json:"draft_id"`
	ChannelID int    `json:"channel_id"` // new id for creates
	Op        string `json:"op"`
	Name      string `json:"name"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
}
