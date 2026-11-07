package nambridge

import (
	"context"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/steven52880/nam/ports"
)

const (
	maxLogRows     = 500
	deleteBatchLen = 1000
)

// logRow carries only the columns nam needs from the logs table.
type logRow struct {
	ID               int64
	CreatedAt        int64
	Type             int
	Content          string
	TokenName        string
	ModelName        string
	Quota            int
	PromptTokens     int
	CompletionTokens int
	UseTime          int
	IsStream         bool
	ChannelID        int    `gorm:"column:channel_id"`
	Group            string `gorm:"column:group"`
	RequestID        string `gorm:"column:request_id"`
	Other            string
}

func groupCol() string {
	if common.LogSqlType == common.DatabaseTypePostgreSQL {
		return `"group"`
	}
	return "`group`"
}

func logColumns() string {
	return "id, created_at, type, content, token_name, model_name, quota, prompt_tokens, " +
		"completion_tokens, use_time, is_stream, channel_id, " + groupCol() + " AS " + groupCol() +
		", request_id, other"
}

func capLimit(n int) int {
	if n <= 0 || n > maxLogRows {
		return maxLogRows
	}
	return n
}

func toRecords(rows []logRow) []ports.LogRecord {
	out := make([]ports.LogRecord, len(rows))
	for i, r := range rows {
		out[i] = ports.LogRecord{
			ID: r.ID, CreatedAt: r.CreatedAt, Type: r.Type, Content: r.Content, TokenName: r.TokenName,
			ModelName: r.ModelName, Quota: r.Quota, PromptTokens: r.PromptTokens,
			CompletionTokens: r.CompletionTokens, UseTime: r.UseTime, IsStream: r.IsStream,
			ChannelID: r.ChannelID, Group: r.Group, RequestID: r.RequestID, Other: r.Other,
		}
	}
	return out
}

// logSource implements ports.LogSource with narrow, index-friendly queries on model.LOG_DB.
// It never calls GetLogPerformance / SumLogStat (full-table aggregations).
type logSource struct{}

func (logSource) FetchAfter(ctx context.Context, q ports.LogQuery) ([]ports.LogRecord, error) {
	db := model.LOG_DB.WithContext(ctx).Table("logs").Select(logColumns()).
		Where("id > ? AND created_at >= ?", q.AfterID, q.Since)
	if len(q.Types) > 0 {
		db = db.Where("type IN ?", q.Types)
	}
	var rows []logRow
	if err := db.Order("id ASC").Limit(capLimit(q.Limit)).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return toRecords(rows), nil
}

func (logSource) Recent(ctx context.Context, channelID, logType int, since int64, limit int) ([]ports.LogRecord, error) {
	var rows []logRow
	err := model.LOG_DB.WithContext(ctx).Table("logs").Select(logColumns()).
		Where("channel_id = ? AND type = ? AND created_at >= ?", channelID, logType, since).
		Order("id DESC").Limit(capLimit(limit)).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return toRecords(rows), nil
}

func (logSource) DeleteBefore(ctx context.Context, before int64) (int64, error) {
	return model.DeleteOldLog(ctx, before, deleteBatchLen)
}
