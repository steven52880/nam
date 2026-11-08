package channels

import (
	"context"

	"github.com/steven52880/nam/internal/store"
)

// MarkDeleteForUpstream drafts the deletion of every channel that belongs to the upstream
// (applied channels become delete drafts; pending create drafts are dropped).
// Returns how many channels were marked.
func (s *Service) MarkDeleteForUpstream(ctx context.Context, upstreamID uint) (int, error) {
	db := s.St.DB.WithContext(ctx)
	var exts []store.ChannelExt
	if err := db.Where("upstream_id = ?", upstreamID).Find(&exts).Error; err != nil {
		return 0, err
	}
	ids := make([]int, 0, len(exts))
	for _, e := range exts {
		ids = append(ids, e.ChannelID)
	}
	var drafts []store.ChannelDraft
	if err := db.Where("op = ?", store.DraftCreate).Find(&drafts).Error; err != nil {
		return 0, err
	}
	for _, d := range drafts {
		if u := parseDraft(d).Ext.UpstreamID; u != nil && *u == upstreamID {
			ids = append(ids, d.ChannelID)
		}
	}
	if len(ids) == 0 {
		return 0, nil
	}
	return len(ids), s.MarkDelete(ctx, ids)
}
