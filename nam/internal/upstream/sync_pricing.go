package upstream

import (
	"context"
	"errors"
	"time"

	"github.com/steven52880/nam/internal/store"
	"gorm.io/gorm"
)

// SyncGroups replaces the upstream's group ratios and per-model prices from adapter
// Pricing. Returns the number of groups. ErrUnsupported is a no-op.
func (s *Syncer) SyncGroups(ctx context.Context, id uint) (int, error) {
	u, site, ad, err := s.load(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return 0, err
		}
		s.record(ctx, id, "", err)
		return 0, err
	}
	p, err := ad.Pricing(ctx, site)
	if errors.Is(err, ErrUnsupported) {
		s.record(ctx, u.ID, u.CredType, nil)
		return 0, nil
	}
	if err == nil {
		err = s.storePricing(ctx, u.ID, p)
	}
	s.record(ctx, u.ID, u.CredType, err)
	if err != nil {
		return 0, err
	}
	return len(p.Groups), nil
}

func (s *Syncer) storePricing(ctx context.Context, id uint, p Pricing) error {
	now := time.Now().Unix()
	groups := make([]store.UpstreamGroup, 0, len(p.Groups))
	seenG := map[string]bool{}
	for _, g := range p.Groups {
		if g.Name == "" || seenG[g.Name] {
			continue
		}
		seenG[g.Name] = true
		groups = append(groups, store.UpstreamGroup{UpstreamID: id, Name: g.Name, Ratio: g.Ratio, Desc: g.Desc, UpdatedAt: now})
	}
	prices := make([]store.UpstreamModelPrice, 0, len(p.Models))
	seenP := map[[2]string]bool{}
	for _, m := range p.Models {
		k := [2]string{m.Group, m.Model}
		if m.Model == "" || seenP[k] {
			continue
		}
		seenP[k] = true
		prices = append(prices, store.UpstreamModelPrice{
			UpstreamID: id, Group: m.Group, Model: m.Model, ModelRatio: m.ModelRatio, Input: m.Input, Output: m.Output,
			CacheRead: m.CacheRead, CacheWrite: m.CacheWrite, PerRequest: m.PerRequest, UpdatedAt: now,
		})
	}
	return s.St.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("upstream_id = ?", id).Delete(&store.UpstreamGroup{}).Error; err != nil {
			return err
		}
		if err := tx.Where("upstream_id = ?", id).Delete(&store.UpstreamModelPrice{}).Error; err != nil {
			return err
		}
		if len(groups) > 0 {
			if err := tx.CreateInBatches(&groups, 200).Error; err != nil {
				return err
			}
		}
		if len(prices) > 0 {
			return tx.CreateInBatches(&prices, 200).Error
		}
		return nil
	})
}

// SyncBalance refreshes balance_usd/balance_at. ErrUnsupported leaves the balance untouched.
// Unlimited balances are stored as NULL.
func (s *Syncer) SyncBalance(ctx context.Context, id uint) error {
	u, site, ad, err := s.load(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return err
		}
		s.record(ctx, id, "", err)
		return err
	}
	b, err := ad.Balance(ctx, site)
	if errors.Is(err, ErrUnsupported) {
		s.record(ctx, u.ID, u.CredType, nil)
		return nil
	}
	if err == nil {
		var usd any
		if !b.Unlimited {
			usd = b.USD
		}
		err = s.St.DB.WithContext(ctx).Model(&store.Upstream{}).Where("id = ?", u.ID).
			Updates(map[string]any{"balance_usd": usd, "balance_at": time.Now().Unix()}).Error
	}
	s.record(ctx, u.ID, u.CredType, err)
	return err
}
