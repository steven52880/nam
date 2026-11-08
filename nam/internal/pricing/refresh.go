package pricing

import (
	"context"
	"encoding/json"
	"time"

	"github.com/steven52880/nam/internal/store"
)

// ChannelInput is what derivation needs about one managed channel.
type ChannelInput struct {
	ChannelID  int
	UpstreamID uint
	Group      string // the token's group on the upstream
	Models     []string
}

// UpstreamPrices loads upstream_model_price for (upstream, group) keyed by model.
func UpstreamPrices(ctx context.Context, st *store.Store, upstreamID uint, group string) (map[string]UpstreamPrice, error) {
	var rows []store.UpstreamModelPrice
	if err := st.DB.WithContext(ctx).Where("upstream_id = ? AND `group` = ?", upstreamID, group).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]UpstreamPrice, len(rows))
	for _, r := range rows {
		out[r.Model] = UpstreamPrice{Input: r.Input, PerRequest: r.PerRequest}
	}
	return out, nil
}

// DeriveFor derives the discount for one channel.
func DeriveFor(ctx context.Context, st *store.Store, book *Book, in ChannelInput) (Derivation, error) {
	up, err := UpstreamPrices(ctx, st, in.UpstreamID, in.Group)
	if err != nil {
		return Derivation{}, err
	}
	return Derive(in.Models, book, up), nil
}

// RefreshResult summarizes an auto-discount refresh.
type RefreshResult struct {
	Updated   []int `json:"updated"`   // channels whose auto discount changed
	Manual    int   `json:"manual"`    // skipped: manual discount
	Underived []int `json:"underived"` // auto but nothing derivable (kept old value)
}

// Change is one discount change, for notifications (M6).
type Change struct {
	ChannelID int     `json:"channel_id"`
	Old       float64 `json:"old"`
	New       float64 `json:"new"`
}

// RefreshAuto recomputes 上游分组折扣 for every channel whose source is "auto".
// Manual discounts are never touched (QA M3 step 2: manual 0.35 survives a sync).
// It writes only nam.db (channel_ext), never NewAPI.
func RefreshAuto(ctx context.Context, st *store.Store, channels []ChannelInput) (RefreshResult, []Change, error) {
	book, err := LoadBook(ctx, st)
	if err != nil {
		return RefreshResult{}, nil, err
	}
	var res RefreshResult
	var changes []Change
	exts := map[int]store.ChannelExt{}
	var rows []store.ChannelExt
	if err := st.DB.WithContext(ctx).Find(&rows).Error; err != nil {
		return res, nil, err
	}
	for _, e := range rows {
		exts[e.ChannelID] = e
	}
	priceCache := map[string]map[string]UpstreamPrice{}
	for _, ch := range channels {
		e, ok := exts[ch.ChannelID]
		if !ok {
			continue
		}
		if e.DiscountSource == store.DiscountManual {
			res.Manual++
			continue
		}
		key := groupKey(ch.UpstreamID, ch.Group)
		up, ok := priceCache[key]
		if !ok {
			if up, err = UpstreamPrices(ctx, st, ch.UpstreamID, ch.Group); err != nil {
				return res, changes, err
			}
			priceCache[key] = up
		}
		d := Derive(ch.Models, book, up)
		if d.Median == nil {
			res.Underived = append(res.Underived, ch.ChannelID)
			continue
		}
		nv := RoundDiscount(*d.Median)
		if nv == e.Discount && e.DiscountSource == store.DiscountAuto {
			continue
		}
		if err := st.DB.WithContext(ctx).Model(&store.ChannelExt{}).Where("channel_id = ?", ch.ChannelID).
			Updates(map[string]any{"discount": nv, "discount_source": store.DiscountAuto, "updated_at": time.Now().Unix()}).Error; err != nil {
			return res, changes, err
		}
		res.Updated = append(res.Updated, ch.ChannelID)
		changes = append(changes, Change{ChannelID: ch.ChannelID, Old: e.Discount, New: nv})
	}
	return res, changes, nil
}

func groupKey(id uint, g string) string {
	b, _ := json.Marshal([]any{id, g})
	return string(b)
}

// ModelDiscounts decodes channel_ext.model_discount (manual per-model overrides).
func ModelDiscounts(raw string) map[string]float64 {
	out := map[string]float64{}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &out)
	}
	return out
}

// EffectiveDiscount is the discount used for a model: per-model override, else channel.
func EffectiveDiscount(channel float64, overrides map[string]float64, model string) float64 {
	if v, ok := overrides[model]; ok && v > 0 {
		return v
	}
	return channel
}
