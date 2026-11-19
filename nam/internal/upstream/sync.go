package upstream

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/steven52880/nam/internal/audit"
	"github.com/steven52880/nam/internal/crypto"
	"github.com/steven52880/nam/internal/store"
	"gorm.io/gorm"
)

// Parts of an upstream that can be synced.
const (
	WhatTokens  = "tokens"
	WhatGroups  = "groups"
	WhatBalance = "balance"
)

// Syncer pulls tokens, group ratios, prices and balances from upstreams into nam.db.
type Syncer struct {
	St    *store.Store
	Box   *crypto.Box
	HTTP  *http.Client
	Audit *audit.Recorder // optional; records manual token creation
}

// load returns the upstream with its decrypted Site and adapter.
func (s *Syncer) load(ctx context.Context, id uint) (store.Upstream, Site, Adapter, error) {
	u, err := s.St.GetUpstream(ctx, id)
	if err != nil {
		return u, Site{}, nil, err
	}
	site, err := SiteFor(u, s.Box)
	if err != nil {
		return u, site, nil, err
	}
	site.HTTP = s.HTTP
	site.Creds = NewCredStore(s.St, s.Box, u.ID)
	ad, err := Get(u.Type)
	return u, site, ad, err
}

// record stores the outcome of one sync on the upstream row, together with the per-part
// outcomes (sync_parts, written atomically with json_set).
// ErrUnsupported counts as success: the operation simply does not exist for this kind.
func (s *Syncer) record(ctx context.Context, id uint, credType string, err error, parts ...namedPart) {
	if errors.Is(err, ErrUnsupported) {
		err = nil
	}
	now := time.Now()
	upd := map[string]any{"last_sync_at": now.Unix(), "updated_at": now}
	if err != nil {
		msg := AuthErrorText(credType, err)
		if r := []rune(msg); len(r) > 500 {
			msg = string(r[:500])
		}
		upd["last_sync_error"] = msg
		upd["consecutive_fail"] = gorm.Expr("consecutive_fail + 1")
	} else {
		upd["last_sync_error"] = ""
		upd["consecutive_fail"] = 0
	}
	if len(parts) > 0 {
		expr, args := partsExpr(parts)
		upd["sync_parts"] = gorm.Expr(expr, args...)
	}
	// Table (not Model): sync_parts is read-only for gorm, so a schema-aware update would skip it.
	if dbErr := s.St.DB.WithContext(context.WithoutCancel(ctx)).Table("upstreams").Where("id = ?", id).Updates(upd).Error; dbErr != nil {
		slog.Warn("nam: record upstream sync", "id", id, "err", dbErr)
	}
}

// Sync runs one part of the sync. The int is the number of tokens/groups stored.
func (s *Syncer) Sync(ctx context.Context, id uint, what string) (int, error) {
	switch what {
	case WhatTokens:
		return s.SyncTokens(ctx, id)
	case WhatGroups:
		return s.SyncGroups(ctx, id)
	case WhatBalance:
		return 0, s.SyncBalance(ctx, id)
	}
	return 0, fmt.Errorf("unknown sync part %q", what)
}

// SyncAll syncs the given parts of every upstream, serially. It returns a one-line
// summary and the number of failed (upstream, part) pairs.
func (s *Syncer) SyncAll(ctx context.Context, what ...string) (string, int) {
	ups, err := s.St.ListUpstreams(ctx)
	if err != nil {
		return "读取上游失败：" + err.Error(), 1
	}
	var fails []string
	for _, u := range ups {
		for _, w := range what {
			if ctx.Err() != nil {
				fails = append(fails, "已取消")
				break
			}
			if _, err := s.Sync(ctx, u.ID, w); err != nil {
				fails = append(fails, fmt.Sprintf("%s/%s: %v", u.Name, w, err))
			}
		}
	}
	summary := fmt.Sprintf("同步 %d 个上游", len(ups))
	if len(fails) > 0 {
		summary += fmt.Sprintf("，%d 项失败：%s", len(fails), strings.Join(fails, "；"))
	}
	return summary, len(fails)
}
