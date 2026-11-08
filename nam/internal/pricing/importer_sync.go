package pricing

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/steven52880/nam/internal/store"
	"gorm.io/gorm"
)

const (
	maxLiteLLMBody = 64 << 20
	litellmTimeout = 60 * time.Second
)

// Sync downloads the LiteLLM price list and applies it to official_price.
func (im *Importer) Sync(ctx context.Context, models []string) (SyncResult, error) {
	prices, err := im.fetch(ctx)
	if err != nil {
		return SyncResult{}, err
	}
	return im.apply(ctx, prices, models)
}

// ImportFile applies a LiteLLM document from r (upload endpoint and tests).
func (im *Importer) ImportFile(ctx context.Context, r io.Reader, models []string) (SyncResult, error) {
	prices, err := ParseLiteLLM(r)
	if err != nil {
		return SyncResult{}, err
	}
	return im.apply(ctx, prices, models)
}

func (im *Importer) fetch(ctx context.Context) (map[string]store.OfficialPrice, error) {
	url := im.LiteLLMURL
	if url == "" {
		url = DefaultLiteLLMURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := im.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch litellm: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch litellm: unexpected status %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxLiteLLMBody+1))
	if err != nil {
		return nil, fmt.Errorf("fetch litellm: read body: %w", err)
	}
	if len(body) > maxLiteLLMBody {
		return nil, fmt.Errorf("fetch litellm: body exceeds %d bytes", maxLiteLLMBody)
	}
	return ParseLiteLLM(bytes.NewReader(body))
}

func (im *Importer) client() *http.Client {
	if im.HTTP != nil {
		return im.HTTP
	}
	return &http.Client{Timeout: litellmTimeout}
}

type namedPrice struct {
	model string
	price store.OfficialPrice
}

func (im *Importer) apply(ctx context.Context, prices map[string]store.OfficialPrice, models []string) (SyncResult, error) {
	res := SyncResult{Fetched: len(prices), Missing: make([]string, 0)}
	if im.St == nil {
		return res, errors.New("pricing: importer requires a store")
	}
	requested := dedupeModels(models)
	lower := lowerNames(prices)
	rows := make([]namedPrice, 0, len(requested))
	for _, m := range requested {
		p, ok := prices[m]
		if !ok {
			if k, found := lower[normalizeName(m)]; found {
				p, ok = prices[k], true
			}
		}
		if !ok {
			res.Missing = append(res.Missing, m)
			continue
		}
		rows = append(rows, namedPrice{model: m, price: p})
	}
	sort.Strings(res.Missing)
	err := im.St.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().Unix()
		for _, rp := range rows {
			p := rp.price
			p.Model = rp.model
			p.UpdatedAt = now
			var cur store.OfficialPrice
			err := tx.First(&cur, "model = ?", p.Model).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := tx.Create(&p).Error; err != nil {
					return err
				}
				res.Updated++
				continue
			}
			if err != nil {
				return err
			}
			if sameOfficialPrice(cur, p) {
				continue
			}
			if err := tx.Model(&store.OfficialPrice{}).Where("model = ?", p.Model).
				Updates(priceUpdates(p)).Error; err != nil {
				return err
			}
			res.Updated++
		}
		q := tx.Where("source <> ?", "manual")
		if len(requested) > 0 {
			q = q.Where("model NOT IN ?", requested)
		}
		return q.Delete(&store.OfficialPrice{}).Error
	})
	if err != nil {
		return res, err
	}
	return res, nil
}

func dedupeModels(models []string) []string {
	seen := make(map[string]bool, len(models))
	out := make([]string, 0, len(models))
	for _, m := range models {
		m = strings.TrimSpace(m)
		if m == "" || seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	return out
}

func lowerNames(prices map[string]store.OfficialPrice) map[string]string {
	keys := make([]string, 0, len(prices))
	for k := range prices {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		lk := normalizeName(k)
		if _, exists := out[lk]; !exists {
			out[lk] = k
		}
	}
	return out
}

func sameOfficialPrice(a, b store.OfficialPrice) bool {
	return a.Vendor == b.Vendor && a.Source == b.Source &&
		samePricePtr(a.Input, b.Input) && samePricePtr(a.Output, b.Output) &&
		samePricePtr(a.CacheRead, b.CacheRead) && samePricePtr(a.CacheWrite5m, b.CacheWrite5m) &&
		samePricePtr(a.CacheWrite1h, b.CacheWrite1h) && samePricePtr(a.PerRequest, b.PerRequest)
}

func samePricePtr(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func priceUpdates(p store.OfficialPrice) map[string]any {
	return map[string]any{
		"vendor":        p.Vendor,
		"input":         p.Input,
		"output":        p.Output,
		"cache_read":    p.CacheRead,
		"cache_write5m": p.CacheWrite5m,
		"cache_write1h": p.CacheWrite1h,
		"per_request":   p.PerRequest,
		"source":        p.Source,
		"updated_at":    p.UpdatedAt,
	}
}
