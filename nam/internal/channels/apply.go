package channels

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Plan returns the apply preview for the given drafts (nil = all).
func (s *Service) Plan(ctx context.Context, ids []int) ([]PlanItem, error) {
	st, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	var out []PlanItem
	for _, d := range st.selectDrafts(ids) {
		out = append(out, st.planItem(d))
	}
	return out, nil
}

func (st *state) selectDrafts(ids []int) []store.ChannelDraft {
	want := map[int]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var ds []store.ChannelDraft
	for id, d := range st.drafts {
		if ids == nil || want[id] {
			ds = append(ds, d)
		}
	}
	// creates last (so renames/deletes free up names first), then by id
	sort.Slice(ds, func(i, j int) bool {
		oi, oj := opOrder(ds[i].Op), opOrder(ds[j].Op)
		if oi != oj {
			return oi < oj
		}
		return ds[i].ChannelID < ds[j].ChannelID
	})
	return ds
}

func opOrder(op string) int {
	switch op {
	case store.DraftDelete:
		return 0
	case store.DraftUpdate:
		return 1
	}
	return 2
}

func (st *state) planItem(d store.ChannelDraft) PlanItem {
	r := st.row(d.ChannelID)
	it := PlanItem{DraftID: d.ID, ChannelID: d.ChannelID, Op: d.Op, Name: r.Name}
	live := st.liveMap[d.ChannelID]
	body := parseDraft(d)
	switch d.Op {
	case store.DraftDelete:
		it.Request = &Request{Method: "DELETE", Path: fmt.Sprintf("/api/channel/%d", d.ChannelID)}
	case store.DraftCreate:
		c := overlay(ports.Channel{}, body.Fields)
		it.Changes = fieldChanges(ports.Channel{}, c, body.Fields)
		create := toCreate(c, "")
		create.Key = "（来自令牌 " + r.TokenName + "）"
		it.Request = &Request{Method: "POST", Path: "/api/channel/", Body: create}
	default:
		c := overlay(live, body.Fields)
		it.Changes = fieldChanges(live, c, body.Fields)
		if !body.Fields.empty() {
			it.Request = &Request{Method: "PUT", Path: "/api/channel/", Body: toPatch(body.Fields)}
		}
	}
	it.Changes = append(it.Changes, extChanges(st.ext[d.ChannelID], body.Ext)...)
	if it.Changes == nil {
		it.Changes = []FieldChange{}
	}
	return it
}

func fieldChanges(old, now ports.Channel, f Fields) []FieldChange {
	var out []FieldChange
	add := func(set bool, name string, o, n any) {
		if set {
			out = append(out, FieldChange{Field: name, Old: o, New: n, NewAPI: true})
		}
	}
	add(f.Name != nil, "name", old.Name, now.Name)
	add(f.BaseURL != nil, "base_url", old.BaseURL, now.BaseURL)
	add(f.Models != nil, "models", old.Models, now.Models)
	add(f.Group != nil, "group", old.Group, now.Group)
	add(f.Priority != nil, "priority", old.Priority, now.Priority)
	add(f.Weight != nil, "weight", old.Weight, now.Weight)
	add(f.Tag != nil, "tag", old.Tag, now.Tag)
	add(f.Status != nil, "status", old.Status, now.Status)
	add(f.TestModel != nil, "test_model", old.TestModel, now.TestModel)
	add(f.Type != nil, "type", old.Type, now.Type)
	return out
}

func extChanges(e store.ChannelExt, x ExtFields) []FieldChange {
	var out []FieldChange
	if x.Discount != nil {
		out = append(out, FieldChange{Field: "discount", Old: e.Discount, New: *x.Discount})
	}
	if x.Vendor != nil {
		out = append(out, FieldChange{Field: "vendor", Old: e.Vendor, New: *x.Vendor})
	}
	if x.UpstreamID != nil {
		out = append(out, FieldChange{Field: "upstream_id", Old: e.UpstreamID, New: *x.UpstreamID})
	}
	if x.TokenID != nil {
		out = append(out, FieldChange{Field: "token_id", Old: e.TokenID, New: *x.TokenID})
	}
	return out
}

func toCreate(c ports.Channel, key string) ports.ChannelCreate {
	st := c.Status
	if st == 0 {
		st = ports.ChannelEnabled
	}
	return ports.ChannelCreate{Type: c.Type, Name: c.Name, Key: key, BaseURL: c.BaseURL, Models: c.Models,
		Group: c.Group, Priority: c.Priority, Weight: c.Weight, Tag: c.Tag, Status: st, TestModel: c.TestModel}
}

func toPatch(f Fields) ports.ChannelPatch {
	return ports.ChannelPatch{Name: f.Name, BaseURL: f.BaseURL, Models: f.Models, Group: f.Group,
		Priority: f.Priority, Weight: f.Weight, Tag: f.Tag, Status: f.Status, TestModel: f.TestModel}
}

// Apply executes drafts serially (nil = all). Failed drafts stay with last_error set.
// Before running it re-validates the whole set; validation failures abort everything.
func (s *Service) Apply(ctx context.Context, ids []int) ([]Result, []ValidationIssue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.load(ctx)
	if err != nil {
		return nil, nil, err
	}
	ds := st.selectDrafts(ids)
	changed := map[int]bool{}
	for _, d := range ds {
		changed[d.ChannelID] = true
	}
	if issues := validateRows(st.rows(), changed, st.vendors); len(issues) > 0 {
		return nil, issues, nil
	}
	results := make([]Result, 0, len(ds))
	for _, d := range ds {
		if ctx.Err() != nil {
			break
		}
		r := s.applyOne(ctx, st, d)
		results = append(results, r)
	}
	return results, nil, nil
}

func (s *Service) applyOne(ctx context.Context, st *state, d store.ChannelDraft) Result {
	row := st.row(d.ChannelID)
	res := Result{DraftID: d.ID, ChannelID: d.ChannelID, Op: d.Op, Name: row.Name}
	body := parseDraft(d)
	var err error
	switch d.Op {
	case store.DraftDelete:
		err = s.applyDelete(ctx, d.ChannelID)
	case store.DraftCreate:
		res.ChannelID, err = s.applyCreate(ctx, st, d, body, row)
	default:
		err = s.applyUpdate(ctx, st, d, body)
	}
	if err != nil {
		res.Error = err.Error()
		s.St.DB.WithContext(context.WithoutCancel(ctx)).Model(&store.ChannelDraft{}).
			Where("id = ?", d.ID).Updates(map[string]any{"last_error": res.Error, "updated_at": time.Now().Unix()})
		return res
	}
	res.OK = true
	return res
}

func (s *Service) applyDelete(ctx context.Context, id int) error {
	if err := s.Channels.Delete(ctx, id); err != nil && !errors.Is(err, ports.ErrNotFound) {
		return err
	}
	return s.St.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("channel_id = ?", id).Delete(&store.ChannelExt{}).Error; err != nil {
			return err
		}
		return tx.Where("channel_id = ?", id).Delete(&store.ChannelDraft{}).Error
	})
}

func (s *Service) applyCreate(ctx context.Context, st *state, d store.ChannelDraft, body draftBody, row Row) (int, error) {
	if row.TokenID == nil {
		return 0, errors.New("新建渠道必须关联令牌")
	}
	key, err := s.tokenKey(st, *row.TokenID)
	if err != nil {
		return 0, err
	}
	c := overlay(ports.Channel{}, body.Fields)
	id, err := s.Channels.Create(ctx, toCreate(c, key))
	if err != nil {
		return 0, err
	}
	ext := store.ChannelExt{ChannelID: id}
	ext = applyExt(ext, body.Ext)
	return id, s.finish(ctx, id, d.ChannelID, ext)
}

func (s *Service) applyUpdate(ctx context.Context, st *state, d store.ChannelDraft, body draftBody) error {
	if !body.Fields.empty() {
		p := toPatch(body.Fields)
		if body.Ext.TokenID != nil {
			key, err := s.tokenKey(st, *body.Ext.TokenID)
			if err != nil {
				return err
			}
			p.Key = &key
		}
		if err := s.Channels.Update(ctx, d.ChannelID, p); err != nil {
			return err
		}
	}
	ext, ok := st.ext[d.ChannelID]
	if !ok {
		ext = store.ChannelExt{ChannelID: d.ChannelID, Discount: 1, DiscountSource: store.DiscountAuto}
	}
	return s.finish(ctx, d.ChannelID, d.ChannelID, applyExt(ext, body.Ext))
}

// finish re-reads the channel, records the applied snapshot and removes the draft.
func (s *Service) finish(ctx context.Context, channelID, draftChannelID int, ext store.ChannelExt) error {
	c, err := s.Channels.Get(ctx, channelID)
	if err != nil {
		return fmt.Errorf("写入成功但读回失败：%w", err)
	}
	sn := snapOf(c)
	ext.AppliedHash, ext.AppliedSnap, ext.DriftFields, ext.UpdatedAt = sn.hash(), sn.json(), "", time.Now().Unix()
	if ext.Discount <= 0 {
		ext.Discount = 1
	}
	if ext.DiscountSource == "" {
		ext.DiscountSource = store.DiscountAuto
	}
	return s.St.DB.WithContext(context.WithoutCancel(ctx)).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&ext).Error; err != nil {
			return err
		}
		return tx.Where("channel_id = ?", draftChannelID).Delete(&store.ChannelDraft{}).Error
	})
}

func applyExt(e store.ChannelExt, x ExtFields) store.ChannelExt {
	if x.UpstreamID != nil {
		e.UpstreamID = x.UpstreamID
	}
	if x.TokenID != nil {
		e.TokenID = x.TokenID
	}
	if x.Vendor != nil {
		e.Vendor = *x.Vendor
	}
	if x.Discount != nil {
		e.Discount = *x.Discount
	}
	if x.DiscountSource != nil {
		e.DiscountSource = *x.DiscountSource
	}
	return e
}

func (s *Service) tokenKey(st *state, tokenID uint) (string, error) {
	t, ok := st.tokens[tokenID]
	if !ok {
		return "", fmt.Errorf("令牌 %d 不存在", tokenID)
	}
	pt, err := s.Box.Open(t.Key)
	if err != nil {
		return "", err
	}
	if len(pt) == 0 {
		return "", fmt.Errorf("令牌 %s 没有保存 key，请先同步或手动录入", t.Name)
	}
	return string(pt), nil
}
