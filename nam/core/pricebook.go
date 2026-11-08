package core

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/steven52880/nam/internal/pricing"
)

// priceBookJob syncs official_price with LiteLLM for the models the site serves
// (union of NewAPI channel models and vendor model lists). Writes nam.db only.
func (c *Core) priceBookJob(ctx context.Context) (string, error) {
	set, err := pricing.LoadSiteModels(ctx, c.store, c.ports.Channels)
	if err != nil {
		return "", fmt.Errorf("读取站内模型失败：%w", err)
	}
	im := pricing.Importer{St: c.store, HTTP: &http.Client{Timeout: 60 * time.Second}, LiteLLMURL: c.cfg.PriceBookURL}
	res, err := im.Sync(ctx, set.Models)
	if err != nil {
		return "", err
	}
	// TODO(M6): notify 价格变动 when a synced price changes.
	return fmt.Sprintf("获取 %d 条，更新 %d 个，缺价 %d 个", res.Fetched, res.Updated, len(res.Missing)), nil
}
