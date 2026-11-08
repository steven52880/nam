package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/steven52880/nam/internal/crypto"
	"github.com/steven52880/nam/internal/pricing"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Service is the channel layer. Channels must be the audit-wrapped ChannelStore.
type Service struct {
	St       *store.Store
	Box      *crypto.Box
	Channels ports.ChannelStore

	// mu serializes every draft mutation and apply: two concurrent "应用" clicks must not
	// create the same channel twice, and edits must not interleave with an apply.
	mu sync.Mutex
}

type state struct {
	live    []ports.Channel
	liveMap map[int]ports.Channel
	ext     map[int]store.ChannelExt
	drafts  map[int]store.ChannelDraft
	ups     map[uint]store.Upstream
	tokens  map[uint]store.UpstreamToken
	vendors *vendorIndex
}

func (s *Service) load(ctx context.Context) (*state, error) {
	live, err := s.Channels.List(ctx)
	if err != nil {
		return nil, resp.Upstream(fmt.Errorf("读取 NewAPI 渠道失败：%w", err))
	}
	st := &state{live: live, liveMap: map[int]ports.Channel{}, ext: map[int]store.ChannelExt{},
		drafts: map[int]store.ChannelDraft{}, ups: map[uint]store.Upstream{}, tokens: map[uint]store.UpstreamToken{}}
	for _, c := range live {
		st.liveMap[c.ID] = c
	}
	db := s.St.DB.WithContext(ctx)
	var exts []store.ChannelExt
	var drafts []store.ChannelDraft
	var ups []store.Upstream
	var toks []store.UpstreamToken
	for _, q := range []struct {
		dst any
	}{{&exts}, {&drafts}, {&ups}, {&toks}} {
		if err := db.Find(q.dst).Error; err != nil {
			return nil, err
		}
	}
	for _, e := range exts {
		st.ext[e.ChannelID] = e
	}
	for _, d := range drafts {
		st.drafts[d.ChannelID] = d
	}
	for _, u := range ups {
		st.ups[u.ID] = u
	}
	for _, t := range toks {
		st.tokens[t.ID] = t
	}
	if st.vendors, err = loadVendors(ctx, s.St); err != nil {
		return nil, err
	}
	return st, nil
}

func parseDraft(d store.ChannelDraft) draftBody {
	var b draftBody
	_ = json.Unmarshal([]byte(d.Diff), &b)
	return b
}

// row builds the UI row for one channel id (live, draft-create, or both).
func (st *state) row(id int) Row {
	live, isLive := st.liveMap[id]
	d, hasDraft := st.drafts[id]
	var body draftBody
	if hasDraft {
		body = parseDraft(d)
	}
	c := overlay(live, body.Fields)
	if !isLive {
		c.ID = id
	}
	r := Row{ID: id, Type: c.Type, Name: c.Name, Status: c.Status, BaseURL: c.BaseURL, Models: c.Models,
		Group: c.Group, Priority: c.Priority, Weight: c.Weight, Tag: c.Tag, TestModel: c.TestModel,
		ResponseTime: live.ResponseTime, TestTime: live.TestTime, UsedQuota: live.UsedQuota, SyncState: StateSynced}
	if r.Models == nil {
		r.Models = []string{}
	}
	e, managed := st.ext[id]
	ext := ExtFields{}
	if managed {
		ext = ExtFields{UpstreamID: e.UpstreamID, TokenID: e.TokenID, Vendor: &e.Vendor, Discount: &e.Discount, DiscountSource: &e.DiscountSource}
	}
	ext = mergeExt(ext, body.Ext)
	r.Managed = managed || !ext.empty()
	r.UpstreamID, r.TokenID = ext.UpstreamID, ext.TokenID
	if ext.Vendor != nil {
		r.Vendor = *ext.Vendor
	}
	r.Discount = 1
	if ext.Discount != nil {
		r.Discount = *ext.Discount
	}
	r.DiscountSource = store.DiscountAuto
	if ext.DiscountSource != nil && *ext.DiscountSource != "" {
		r.DiscountSource = *ext.DiscountSource
	}
	if r.UpstreamID != nil {
		if u, ok := st.ups[*r.UpstreamID]; ok {
			r.UpstreamName = u.Name
			rate := u.CurrencyRate
			cost := r.Discount * rate
			r.CurrencyRate, r.ChannelCost = &rate, &cost
		}
	}
	if r.TokenID != nil {
		if t, ok := st.tokens[*r.TokenID]; ok {
			r.TokenName = t.Name
		}
	}
	if managed && e.DriftFields != "" {
		r.DriftFields = splitComma(e.DriftFields)
		r.SyncState = StateDrift
	}
	if hasDraft {
		r.DraftID, r.DraftOp, r.DraftError = d.ID, d.Op, d.LastError
		r.DirtyFields = dirtyNames(body.Fields, body.Ext)
		switch {
		case d.LastError != "":
			r.SyncState = StateError
		case d.Op == store.DraftCreate:
			r.SyncState = StateDraftCreate
		case d.Op == store.DraftDelete:
			r.SyncState = StateDraftDelete
		default:
			r.SyncState = StateDraft
		}
	}
	if r.DirtyFields == nil {
		r.DirtyFields = []string{}
	}
	if r.DriftFields == nil {
		r.DriftFields = []string{}
	}
	return r
}

func (st *state) rows() []Row {
	ids := make([]int, 0, len(st.liveMap)+len(st.drafts))
	for id := range st.liveMap {
		ids = append(ids, id)
	}
	for id, d := range st.drafts {
		if _, ok := st.liveMap[id]; !ok && d.Op == store.DraftCreate {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	out := make([]Row, 0, len(ids))
	for _, id := range ids {
		out = append(out, st.row(id))
	}
	return out
}

// List returns every channel row (NewAPI channels + pending creates).
func (s *Service) List(ctx context.Context) ([]Row, error) {
	st, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	rows := st.rows()
	book, err := pricing.LoadBook(ctx, s.St)
	if err != nil {
		return nil, err
	}
	priceRows(rows, st, book)
	return rows, nil
}

// DraftCount returns the number of pending drafts.
func (s *Service) DraftCount(ctx context.Context) (int64, error) {
	var n int64
	err := s.St.DB.WithContext(ctx).Model(&store.ChannelDraft{}).Count(&n).Error
	return n, err
}

// nextTempID returns a new negative id for a create draft.
func nextTempID(tx *gorm.DB) (int, error) {
	var minID *int
	if err := tx.Model(&store.ChannelDraft{}).Select("MIN(channel_id)").Scan(&minID).Error; err != nil {
		return 0, err
	}
	if minID == nil || *minID >= 0 {
		return -1, nil
	}
	return *minID - 1, nil
}

// SaveChanges merges edits into drafts after validating the resulting channel set.
// Returns validation issues (and saves nothing) if any.
func (s *Service) SaveChanges(ctx context.Context, changes []Change) ([]ValidationIssue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveChanges(ctx, changes)
}

func (s *Service) saveChanges(ctx context.Context, changes []Change) ([]ValidationIssue, error) {
	st, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	changed := map[int]bool{}
	pending := map[int]store.ChannelDraft{}
	for _, ch := range changes {
		id := ch.ChannelID
		d, has := st.drafts[id]
		if p, ok := pending[id]; ok {
			d, has = p, true
		}
		live, isLive := st.liveMap[id]
		if !isLive && (!has || d.Op != store.DraftCreate) {
			return nil, resp.Invalid(fmt.Sprintf("渠道 %d 不存在", id))
		}
		if has && d.Op == store.DraftDelete {
			return nil, resp.Invalid(fmt.Sprintf("渠道 %d 已标记删除，先撤销删除再修改", id))
		}
		body := draftBody{}
		if has {
			body = parseDraft(d)
		}
		body.Fields = mergeFields(body.Fields, ch.Fields)
		body.Ext = mergeExt(body.Ext, ch.Ext)
		if ch.Ext.Discount != nil && ch.Ext.DiscountSource == nil {
			m := store.DiscountManual
			body.Ext.DiscountSource = &m
		}
		if isLive {
			body.Fields = pruneFields(live, body.Fields)
			body.Ext = pruneExt(st.ext[id], st.ext[id].ChannelID != 0, body.Ext)
		}
		if !has {
			d = store.ChannelDraft{ChannelID: id, Op: store.DraftUpdate, CreatedAt: time.Now().Unix()}
		}
		raw, _ := json.Marshal(body)
		d.Diff, d.LastError, d.UpdatedAt = string(raw), "", time.Now().Unix()
		pending[id] = d
		st.drafts[id] = d
		changed[id] = true
	}
	if issues := validateRows(st.rows(), changed, st.vendors); len(issues) > 0 {
		return issues, nil
	}
	return nil, s.St.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, d := range pending {
			body := parseDraft(d)
			if d.Op == store.DraftUpdate && body.Fields.empty() && body.Ext.empty() {
				if err := tx.Where("channel_id = ?", d.ChannelID).Delete(&store.ChannelDraft{}).Error; err != nil {
					return err
				}
				continue
			}
			if err := upsertDraft(tx, &d); err != nil {
				return err
			}
		}
		return nil
	})
}

func pruneExt(e store.ChannelExt, exists bool, x ExtFields) ExtFields {
	if !exists {
		return x
	}
	if x.UpstreamID != nil && e.UpstreamID != nil && *x.UpstreamID == *e.UpstreamID {
		x.UpstreamID = nil
	}
	if x.TokenID != nil && e.TokenID != nil && *x.TokenID == *e.TokenID {
		x.TokenID = nil
	}
	if x.Vendor != nil && *x.Vendor == e.Vendor {
		x.Vendor = nil
	}
	if x.Discount != nil && *x.Discount == e.Discount {
		x.Discount = nil
		x.DiscountSource = nil
	}
	if x.DiscountSource != nil && *x.DiscountSource == e.DiscountSource && x.Discount == nil {
		x.DiscountSource = nil
	}
	return x
}

func upsertDraft(tx *gorm.DB, d *store.ChannelDraft) error {
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "channel_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"op", "diff", "last_error", "updated_at"}),
	}).Create(d).Error
}

// MarkDelete adds delete drafts (create drafts are simply dropped).
func (s *Service) MarkDelete(ctx context.Context, ids []int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.St.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			if id < 0 {
				if err := tx.Where("channel_id = ?", id).Delete(&store.ChannelDraft{}).Error; err != nil {
					return err
				}
				continue
			}
			d := store.ChannelDraft{ChannelID: id, Op: store.DraftDelete, Diff: "{}", CreatedAt: time.Now().Unix(), UpdatedAt: time.Now().Unix()}
			if err := upsertDraft(tx, &d); err != nil {
				return err
			}
		}
		return nil
	})
}

// Discard removes drafts; nil ids = all.
func (s *Service) Discard(ctx context.Context, ids []int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := s.St.DB.WithContext(ctx)
	if ids == nil {
		return q.Where("1 = 1").Delete(&store.ChannelDraft{}).Error
	}
	return q.Where("channel_id IN ?", ids).Delete(&store.ChannelDraft{}).Error
}

// ErrDraftNotFound is returned when applying unknown drafts.
var ErrDraftNotFound = errors.New("draft not found")
