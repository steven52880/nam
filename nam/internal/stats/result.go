package stats

// Query windows, dimensions and buckets accepted by the stats queries.
const (
	WindowLatest = "latest"
	Window3d     = "3d"
	Window7d     = "7d"

	DimChannel  = "channel"
	DimModel    = "model"
	DimUpstream = "upstream"
	DimVendor   = "vendor"

	BucketHour = "hour"
	BucketDay  = "day"

	// MaxSpanSeconds is the longest from–to span the overview and series accept (186 days).
	MaxSpanSeconds = 186 * 86400
)

// Price and quality tiers.
const (
	TierCheap     = "cheap"
	TierMid       = "mid"
	TierExpensive = "expensive"

	QualityGood         = "good"
	QualityMid          = "mid"
	QualityBad          = "bad"
	QualityInsufficient = "insufficient"
)

// ChannelStatsItem is one channel row of GET /stats/channels (Metrics fields are inlined).
type ChannelStatsItem struct {
	Metrics
	ChannelID    int      `json:"channel_id"`
	StdCostPrice *float64 `json:"std_cost_price"`
	PriceTier    *string  `json:"price_tier"`
	QualityTier  string   `json:"quality_tier"`
}

// ChannelStatsList is the GET /stats/channels payload.
type ChannelStatsList struct {
	Window       string             `json:"window"`
	GeneratedAt  int64              `json:"generated_at"`
	PatchP1      bool               `json:"patch_p1"`
	ErrorLogHint bool               `json:"error_log_hint"`
	Items        []ChannelStatsItem `json:"items"`
}

// ModelStatsItem is one model row of GET /stats/channels/:id/models.
type ModelStatsItem struct {
	Metrics
	Model string `json:"model"`
}

// ModelStatsList is the GET /stats/channels/:id/models payload.
type ModelStatsList struct {
	Items []ModelStatsItem `json:"items"`
}

// StatsRow is one ranking row of the overview.
type StatsRow struct {
	Metrics
	Key  string `json:"key"`
	Name string `json:"name"`
}

// ScatterPoint is one channel of the price–quality scatter.
type ScatterPoint struct {
	ChannelID    int      `json:"channel_id"`
	Name         string   `json:"name"`
	Vendor       string   `json:"vendor"`
	StdCostPrice *float64 `json:"std_cost_price"`
	Wilson       *float64 `json:"wilson"`
	Requests     int64    `json:"requests"`
}

// OverviewResult is the GET /stats/overview payload.
type OverviewResult struct {
	From         int64            `json:"from"`
	To           int64            `json:"to"`
	Dim          string           `json:"dim"`
	PatchP1      bool             `json:"patch_p1"`
	ErrorLogHint bool             `json:"error_log_hint"`
	Totals       Metrics          `json:"totals"`
	Rows         []StatsRow       `json:"rows"`
	ErrorClasses map[string]int64 `json:"error_classes"`
	Scatter      []ScatterPoint   `json:"scatter"`
}

// SeriesPoint is one bucket of GET /stats/series.
type SeriesPoint struct {
	Metrics
	T int64 `json:"t"`
}

// SeriesResult is the GET /stats/series payload.
type SeriesResult struct {
	Bucket string        `json:"bucket"`
	Points []SeriesPoint `json:"points"`
}
