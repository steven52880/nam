package store

// StatsHourly is aggregated per channel/model/hour. Never stores raw logs.
type StatsHourly struct {
	ChannelID int    `gorm:"primaryKey;autoIncrement:false" json:"channel_id"`
	Model     string `gorm:"primaryKey;size:128" json:"model"`
	Hour      int64  `gorm:"primaryKey;autoIncrement:false;index" json:"hour"` // unix seconds, truncated to hour
	Group     string `gorm:"size:64" json:"group"`
	Vendor    string `gorm:"size:64;index" json:"vendor"` // fixed at ingest: vendor by group, else channel_ext.vendor

	Success     int64  `json:"success"`
	Fail        int64  `json:"fail"`
	FailByClass string `json:"fail_by_class"` // JSON {"429":n,"5xx":n,"timeout":n,"401":n,"balance":n,"other":n}

	// PromptTokens is NON-CACHED input only (prompt minus cache read/write, Anthropic
	// semantics normalised), not NewAPI's raw prompt_tokens.
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	CacheReadTokens  int64 `json:"cache_read_tokens"`
	CacheWriteTokens int64 `json:"cache_write_tokens"` // 5m + 1h

	FRTHist      string `json:"frt_hist"`       // log histogram JSON (ms)
	SpeedHist    string `json:"speed_hist"`     // log histogram JSON (token/s), exact (use_time_ms)
	SpeedEstHist string `json:"speed_est_hist"` // log histogram JSON (token/s), estimated from whole seconds

	CostCNY            float64 `json:"cost_cny"`
	OfficialUSD        float64 `json:"official_usd"`          // official price with official cache prices
	OfficialNoCacheUSD float64 `json:"official_no_cache_usd"` // official price as if nothing was cached
	PricedRequests     int64   `json:"priced_requests"`       // successes whose cost could be computed
	Quota              int64   `json:"quota"`
	RevenueCNY         float64 `json:"revenue_cny"`
}

// LogCursor tracks incremental log fetching. Only one row is used: LogType = 0
// (consume + error merged).
type LogCursor struct {
	LogType int   `gorm:"primaryKey;autoIncrement:false" json:"log_type"`
	LastID  int64 `json:"last_id"`
	LastAt  int64 `json:"last_at"`
}

// Stats setting keys.
const (
	KeyStatsBackfillDays   = "stats_backfill_days"     // 首次回填天数，默认 7（1–90）
	KeyNewAPILogCleanup    = "newapi_log_cleanup"      // 清理 NewAPI 原始日志开关，默认 false
	KeyStatsPatchP1Seen    = "stats_patch_p1_seen"     // internal: a consume log carried use_time_ms
	KeyStatsErrorLogSeenAt = "stats_error_log_seen_at" // internal: newest error log created_at seen
	KeyStatsRebuildSince   = "stats_rebuild_since"     // internal: since for the first pull after rebuild
)
