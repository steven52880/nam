package channels

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

// DriftReport summarizes one consistency check.
type DriftReport struct {
	Checked      int   `json:"checked"`
	Drifted      []int `json:"drifted"`
	Vanished     []int `json:"vanished"`      // ext rows whose channel no longer exists in NewAPI
	AutoDisabled []int `json:"auto_disabled"` // managed channels NewAPI auto-disabled (status 3)
}

// CheckDrift compares every managed channel with its applied snapshot and stores
// drift_fields. It never writes NewAPI.
func (s *Service) CheckDrift(ctx context.Context) (DriftReport, error) {
	st, err := s.load(ctx)
	if err != nil {
		return DriftReport{}, err
	}
	rep := DriftReport{Drifted: []int{}, Vanished: []int{}, AutoDisabled: []int{}}
	db := s.St.DB.WithContext(ctx)
	for id, e := range st.ext {
		live, ok := st.liveMap[id]
		if !ok {
			rep.Vanished = append(rep.Vanished, id)
			continue
		}
		rep.Checked++
		if live.Status == ports.ChannelAutoDisabled {
			rep.AutoDisabled = append(rep.AutoDisabled, id)
		}
		applied, ok := parseSnap(e.AppliedSnap)
		if !ok {
			continue // adopted channels without snapshot: nothing to compare
		}
		fields := strings.Join(diffSnap(applied, snapOf(live), st.vendors.isManaged), ",")
		if fields != "" {
			rep.Drifted = append(rep.Drifted, id)
		}
		if fields != e.DriftFields {
			if err := db.Model(&store.ChannelExt{}).Where("channel_id = ?", id).
				Updates(map[string]any{"drift_fields": fields, "updated_at": time.Now().Unix()}).Error; err != nil {
				return rep, err
			}
		}
	}
	return rep, nil
}

// DriftDetail is the per-field comparison for the drift dialog.
type DriftDetail struct {
	ChannelID int           `json:"channel_id"`
	Name      string        `json:"name"`
	Fields    []FieldChange `json:"fields"` // Old = nam last applied, New = NewAPI now
}

// Drift returns the comparison for one channel.
func (s *Service) Drift(ctx context.Context, id int) (DriftDetail, error) {
	st, err := s.load(ctx)
	if err != nil {
		return DriftDetail{}, err
	}
	live, ok := st.liveMap[id]
	if !ok {
		return DriftDetail{}, ports.ErrNotFound
	}
	applied, _ := parseSnap(st.ext[id].AppliedSnap)
	now := snapOf(live)
	out := DriftDetail{ChannelID: id, Name: live.Name, Fields: []FieldChange{}}
	vals := func(s snapshot) map[string]any {
		// show only the managed groups: those are what can drift
		group := joinGroups(managedSubset(splitGroups(s.Group), st.vendors.isManaged))
		return map[string]any{"models": s.Models, "group": group, "priority": s.Priority, "weight": s.Weight,
			"tag": s.Tag, "status": s.Status, "test_model": s.TestModel, "base_url": s.BaseURL}
	}
	a, n := vals(applied), vals(now)
	for _, f := range diffSnap(applied, now, st.vendors.isManaged) {
		out.Fields = append(out.Fields, FieldChange{Field: f, Old: a[f], New: n[f], NewAPI: true})
	}
	return out, nil
}

// AcceptDrift adopts NewAPI's current values (updates nam's snapshot only).
func (s *Service) AcceptDrift(ctx context.Context, id int) error {
	c, err := s.Channels.Get(ctx, id)
	if err != nil {
		return err
	}
	sn := snapOf(c)
	res := s.St.DB.WithContext(ctx).Model(&store.ChannelExt{}).Where("channel_id = ?", id).
		Updates(map[string]any{"applied_hash": sn.hash(), "applied_snap": sn.json(), "drift_fields": "", "updated_at": time.Now().Unix()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ports.ErrNotFound
	}
	return nil
}

// OverwriteDrift creates an update draft that writes nam's last applied values back.
// The caller applies it through the normal draft flow (preview + confirm).
func (s *Service) OverwriteDrift(ctx context.Context, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.load(ctx)
	if err != nil {
		return err
	}
	applied, ok := parseSnap(st.ext[id].AppliedSnap)
	if !ok {
		return resp.Invalid(fmt.Sprintf("渠道 %d 没有 nam 上次应用的记录", id))
	}
	if _, live := st.liveMap[id]; !live {
		return ports.ErrNotFound
	}
	f := Fields{Models: &applied.Models, Group: &applied.Group, Priority: &applied.Priority, Weight: &applied.Weight,
		Tag: &applied.Tag, Status: &applied.Status, TestModel: &applied.TestModel, BaseURL: &applied.BaseURL}
	issues, err := s.saveChanges(ctx, []Change{{ChannelID: id, Fields: f}})
	if err != nil {
		return err
	}
	if len(issues) > 0 {
		return resp.Invalid("覆盖后的值不合法：" + issues[0].Message)
	}
	return nil
}

// Adopt starts managing an existing NewAPI channel (creates its ext row with a snapshot).
func (s *Service) Adopt(ctx context.Context, id int, ext ExtFields) error {
	c, err := s.Channels.Get(ctx, id)
	if err != nil {
		return err
	}
	e := applyExt(store.ChannelExt{ChannelID: id, Discount: 1, DiscountSource: store.DiscountAuto}, ext)
	sn := snapOf(c)
	e.AppliedHash, e.AppliedSnap, e.UpdatedAt = sn.hash(), sn.json(), time.Now().Unix()
	return s.St.DB.WithContext(ctx).Save(&e).Error
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
