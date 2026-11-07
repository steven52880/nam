package store

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// ErrNotFound is returned by lookups that find nothing.
var ErrNotFound = errors.New("not found")

func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

// ListUpstreams returns all upstreams ordered by name.
func (s *Store) ListUpstreams(ctx context.Context) ([]Upstream, error) {
	var out []Upstream
	err := s.DB.WithContext(ctx).Order("name").Find(&out).Error
	return out, err
}

// GetUpstream returns one upstream.
func (s *Store) GetUpstream(ctx context.Context, id uint) (Upstream, error) {
	var u Upstream
	err := s.DB.WithContext(ctx).First(&u, id).Error
	return u, notFound(err)
}

// SaveUpstream creates or updates an upstream.
func (s *Store) SaveUpstream(ctx context.Context, u *Upstream) error {
	return s.DB.WithContext(ctx).Save(u).Error
}

// DeleteUpstream deletes an upstream and its synced data (groups, tokens, prices).
// Channel ext rows keep pointing to the id; the caller drafts channel deletions.
func (s *Store) DeleteUpstream(ctx context.Context, id uint) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, m := range []any{&UpstreamGroup{}, &UpstreamToken{}, &UpstreamModelPrice{}} {
			if err := tx.Where("upstream_id = ?", id).Delete(m).Error; err != nil {
				return err
			}
		}
		res := tx.Delete(&Upstream{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// UpstreamCounts returns token and channel counts per upstream id.
func (s *Store) UpstreamCounts(ctx context.Context) (tokens, channels map[uint]int, err error) {
	type row struct {
		ID uint
		N  int
	}
	var tr, cr []row
	if err = s.DB.WithContext(ctx).Model(&UpstreamToken{}).Select("upstream_id as id, count(*) as n").Group("upstream_id").Scan(&tr).Error; err != nil {
		return
	}
	if err = s.DB.WithContext(ctx).Model(&ChannelExt{}).Where("upstream_id is not null").Select("upstream_id as id, count(*) as n").Group("upstream_id").Scan(&cr).Error; err != nil {
		return
	}
	tokens, channels = map[uint]int{}, map[uint]int{}
	for _, r := range tr {
		tokens[r.ID] = r.N
	}
	for _, r := range cr {
		channels[r.ID] = r.N
	}
	return
}

// AuditFilter filters audit_log listing.
type AuditFilter struct {
	Kind    string
	Trigger string
	Result  string
	From    int64
	To      int64
	Q       string
	Limit   int
	Offset  int
}

// ListAudit returns audit rows newest first and the total count.
func (s *Store) ListAudit(ctx context.Context, f AuditFilter) ([]AuditLog, int64, error) {
	q := s.DB.WithContext(ctx).Model(&AuditLog{})
	if f.Kind != "" {
		q = q.Where("kind = ?", f.Kind)
	}
	if f.Trigger != "" {
		q = q.Where("trigger = ?", f.Trigger)
	}
	if f.Result != "" {
		q = q.Where("result = ?", f.Result)
	}
	if f.From > 0 {
		q = q.Where("at >= ?", f.From)
	}
	if f.To > 0 {
		q = q.Where("at <= ?", f.To)
	}
	if f.Q != "" {
		like := "%" + f.Q + "%"
		q = q.Where("target LIKE ? OR summary LIKE ?", like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 50
	}
	var out []AuditLog
	err := q.Order("id desc").Limit(f.Limit).Offset(f.Offset).Find(&out).Error
	return out, total, err
}

// GetAudit returns one audit row.
func (s *Store) GetAudit(ctx context.Context, id uint) (AuditLog, error) {
	var a AuditLog
	err := s.DB.WithContext(ctx).First(&a, id).Error
	return a, notFound(err)
}
