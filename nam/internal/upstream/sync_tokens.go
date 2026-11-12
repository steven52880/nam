package upstream

import (
	"context"
	"errors"
	"time"

	"github.com/steven52880/nam/internal/store"
	"gorm.io/gorm"
)

// keyHint is the display form of a key: first 5 + … + last 4 (shorter keys show less).
func keyHint(key string) string {
	r := []rune(key)
	switch {
	case len(r) <= 2:
		return "…"
	case len(r) <= 9:
		return string(r[:2]) + "…"
	}
	return string(r[:5]) + "…" + string(r[len(r)-4:])
}

// KeyHint is the exported display form of a key (see keyHint).
func KeyHint(key string) string { return keyHint(key) }

// SyncTokens pulls the token list and upserts it. Returns the number of remote tokens.
func (s *Syncer) SyncTokens(ctx context.Context, id uint) (int, error) {
	u, site, ad, err := s.load(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return 0, err
		}
		s.record(ctx, id, "", err)
		return 0, err
	}
	remote, err := ad.ListTokens(ctx, site)
	if errors.Is(err, ErrUnsupported) {
		s.record(ctx, u.ID, u.CredType, nil)
		return 0, nil
	}
	if err == nil {
		err = s.storeTokens(ctx, u.ID, remote)
	}
	s.record(ctx, u.ID, u.CredType, err)
	if err != nil {
		return 0, err
	}
	return len(remote), nil
}

// storeTokens upserts remote tokens by (upstream, remote_id) — or by name for single-key
// adapters whose remote_id is empty — and retires tokens that disappeared remotely.
func (s *Syncer) storeTokens(ctx context.Context, id uint, remote []Token) error {
	now := time.Now().Unix()
	return s.St.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing []store.UpstreamToken
		if err := tx.Where("upstream_id = ?", id).Find(&existing).Error; err != nil {
			return err
		}
		byRemote, byName := map[string]int{}, map[string]int{}
		for i, t := range existing {
			switch {
			case t.Manual:
			case t.RemoteID != "":
				byRemote[t.RemoteID] = i
			default:
				byName[t.Name] = i
			}
		}
		seen := map[int]bool{}
		for _, rt := range remote {
			idx, found := byName[rt.Name]
			if rt.RemoteID != "" {
				idx, found = byRemote[rt.RemoteID]
			}
			cur := store.UpstreamToken{UpstreamID: id}
			if found {
				cur = existing[idx]
				seen[idx] = true
			}
			cur.RemoteID, cur.Name, cur.Group = rt.RemoteID, rt.Name, rt.Group
			cur.QuotaUSD, cur.Unlimited, cur.Status, cur.UpdatedAt = rt.QuotaUSD, rt.Unlimited, rt.Status, now
			if rt.Key != "" { // some upstreams never reveal the key again: keep what we have
				sealed, err := s.Box.Seal([]byte(rt.Key))
				if err != nil {
					return err
				}
				cur.Key, cur.KeyHint = sealed, keyHint(rt.Key)
			}
			if err := tx.Save(&cur).Error; err != nil {
				return err
			}
		}
		for i, t := range existing {
			if t.Manual || seen[i] {
				continue
			}
			var refs int64
			if err := tx.Model(&store.ChannelExt{}).Where("token_id = ?", t.ID).Count(&refs).Error; err != nil {
				return err
			}
			if refs > 0 { // channels still use it: keep, but mark gone
				if t.Status != 0 {
					if err := tx.Model(&store.UpstreamToken{}).Where("id = ?", t.ID).
						Updates(map[string]any{"status": 0, "updated_at": now}).Error; err != nil {
						return err
					}
				}
				continue
			}
			if err := tx.Delete(&store.UpstreamToken{}, t.ID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
