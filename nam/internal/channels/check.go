package channels

import (
	"context"
	"time"

	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

// Adopt starts managing an existing NewAPI channel (creates its ext row; no snapshot).
func (s *Service) Adopt(ctx context.Context, id int, ext ExtFields) error {
	if _, err := s.Channels.Get(ctx, id); err != nil {
		return err
	}
	e := applyExt(store.ChannelExt{ChannelID: id}, normExt(ext))
	e.UpdatedAt = time.Now().Unix()
	return s.St.DB.WithContext(ctx).Save(&e).Error
}

// CheckReport summarizes one rule check.
type CheckReport struct {
	Checked      int     `json:"checked"`       // channels checked (drafts overlaid, deleted drafts skipped)
	Issues       []Issue `json:"issues"`        // rule problems
	AutoDisabled []int   `json:"auto_disabled"` // channels NewAPI auto-disabled (status 3)
}

// Check runs the validateRows rules over every channel (drafts overlaid) and lists the
// channels NewAPI auto-disabled. It never writes.
func (s *Service) Check(ctx context.Context) (CheckReport, error) {
	st, err := s.load(ctx)
	if err != nil {
		return CheckReport{}, err
	}
	rows := st.rows()
	changed := make(map[int]bool, len(rows))
	rep := CheckReport{AutoDisabled: []int{}}
	for _, r := range rows {
		if r.DraftOp == store.DraftDelete {
			continue
		}
		changed[r.ID] = true
		rep.Checked++
		if r.Status == ports.ChannelAutoDisabled {
			rep.AutoDisabled = append(rep.AutoDisabled, r.ID)
		}
	}
	rep.Issues = validateRows(rows, changed, st.vendors)
	if rep.Issues == nil {
		rep.Issues = []Issue{}
	}
	return rep, nil
}

// Issues is GET /channels/issues: the rule problems of every channel.
func (s *Service) Issues(ctx context.Context) ([]Issue, error) {
	rep, err := s.Check(ctx)
	return rep.Issues, err
}
