package channels

import (
	"context"
	"encoding/json"
	"time"

	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/internal/vendors"
	"gorm.io/gorm"
)

// UpdateVendor edits a vendor and carries the change over to channels. A vendor rename
// updates channel_ext and draft ext.vendor; a group rename creates update drafts that
// swap the old group for the new one on every non-deleted channel carrying it. Nothing
// is written to NewAPI: the user applies the drafts from the channel page.
func (s *Service) UpdateVendor(ctx context.Context, id uint, in vendors.Input) (VendorUpdate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// load before the transaction: the store allows a single connection
	st, err := s.load(ctx)
	if err != nil {
		return VendorUpdate{}, err
	}
	var out VendorUpdate
	err = s.St.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		out = VendorUpdate{}
		old, cur, err := vendors.UpdateTx(tx, id, in)
		if err != nil {
			return err
		}
		out.Vendor = cur
		touched := map[int]store.ChannelDraft{}
		if old.Name != cur.Name {
			if err := tx.Model(&store.ChannelExt{}).Where("vendor = ?", old.Name).Update("vendor", cur.Name).Error; err != nil {
				return err
			}
			for chID, d := range st.drafts {
				body := parseDraft(d)
				if body.Ext.Vendor == nil || *body.Ext.Vendor != old.Name {
					continue
				}
				body.Ext.Vendor = &cur.Name
				raw, _ := json.Marshal(body)
				d.Diff = string(raw)
				st.drafts[chID] = d
				touched[chID] = d
				out.RenamedDrafts++
			}
		}
		if old.GroupName != cur.GroupName {
			for _, r := range st.rows() {
				if r.DraftOp == store.DraftDelete {
					continue
				}
				gs := splitGroups(r.Group)
				if !containsGroup(gs, old.GroupName) {
					continue
				}
				d, has := st.drafts[r.ID]
				if !has {
					d = store.ChannelDraft{ChannelID: r.ID, Op: store.DraftUpdate, CreatedAt: time.Now().Unix()}
				}
				body := parseDraft(d)
				g := joinGroups(renameGroup(gs, old.GroupName, cur.GroupName))
				body.Fields.Group = &g
				if _, isLive := st.liveMap[r.ID]; isLive && !containsGroup(body.GroupDrop, old.GroupName) {
					body.GroupDrop = append(body.GroupDrop, old.GroupName)
				}
				raw, _ := json.Marshal(body)
				d.Diff = string(raw)
				st.drafts[r.ID] = d
				touched[r.ID] = d
				out.GroupDrafts++
			}
		}
		now := time.Now().Unix()
		for _, d := range touched {
			d.LastError, d.UpdatedAt = "", now
			if err := upsertDraft(tx, &d); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return VendorUpdate{}, err
	}
	return out, nil
}
