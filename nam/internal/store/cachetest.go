package store

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// Cache test statuses.
const (
	CacheTestRunning = "running"
	CacheTestOK      = "ok"
	CacheTestFail    = "fail"
)

// Cache test modes.
const (
	CacheTestModeDirect = "direct" // nam → upstream, bypassing NewAPI
	CacheTestModeNewAPI = "newapi" // nam → NewAPI relay with a channel-pinned key
)

// AuditCacheTest is the audit kind of a manual cache-rate test.
const AuditCacheTest = "cache_test"

// Settings used by the manual cache-rate test.
const (
	// KeyCacheTestToken holds the sealed (base64) NewAPI admin token. Never exposed.
	KeyCacheTestToken = "cache_test.token"
	// KeyCacheTestTokenHint is the display hint of that token (safe to expose).
	KeyCacheTestTokenHint = "cache_test.token_hint"
	// KeyNewAPIURL overrides the NewAPI /v1 base address used by the test.
	KeyNewAPIURL = "newapi_url"
)

// CacheTest is one manual cache-rate test run. History is kept; lists use the latest per channel.
type CacheTest struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	ChannelID int    `gorm:"index;not null" json:"channel_id"`
	Model     string `gorm:"size:128;not null" json:"model"`
	Protocol  string `gorm:"size:16" json:"protocol"`           // anthropic / openai
	Mode      string `gorm:"size:8;default:direct" json:"mode"` // direct / newapi
	Status    string `gorm:"size:8;not null" json:"status"`
	Rounds    int    `json:"rounds"`
	// The three token sums cover rounds 2..N only. InputTokens is the TOTAL input
	// (provider input + cache read + cache write for Anthropic; prompt_tokens for OpenAI).
	InputTokens      int64    `json:"input_tokens"`
	CacheReadTokens  int64    `json:"cache_read_tokens"`
	CacheWriteTokens int64    `json:"cache_write_tokens"`
	Rate             *float64 `json:"rate"`
	PerRound         string   `json:"per_round"` // JSON array of per-round usage
	Error            string   `json:"error"`
	StartedAt        int64    `json:"started_at"`
	DurationMs       int64    `json:"duration_ms"`
}

// CreateCacheTest inserts a new run.
func (s *Store) CreateCacheTest(ctx context.Context, t *CacheTest) error {
	return s.DB.WithContext(ctx).Create(t).Error
}

// UpdateCacheTest updates the given columns of a run.
func (s *Store) UpdateCacheTest(ctx context.Context, id uint, fields map[string]any) error {
	return s.DB.WithContext(ctx).Model(&CacheTest{}).Where("id = ?", id).Updates(fields).Error
}

// GetCacheTest returns one run or ErrNotFound.
func (s *Store) GetCacheTest(ctx context.Context, id uint) (CacheTest, error) {
	var t CacheTest
	err := s.DB.WithContext(ctx).First(&t, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CacheTest{}, ErrNotFound
	}
	return t, err
}

// LatestCacheTests returns the newest run of every channel.
func (s *Store) LatestCacheTests(ctx context.Context) ([]CacheTest, error) {
	var out []CacheTest
	err := s.DB.WithContext(ctx).
		Where("id IN (?)", s.DB.Model(&CacheTest{}).Select("MAX(id)").Group("channel_id")).
		Order("channel_id").Find(&out).Error
	return out, err
}
