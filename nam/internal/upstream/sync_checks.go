package upstream

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/steven52880/nam/internal/store"
)

// CheckResult is one connectivity check of the "add upstream" wizard.
type CheckResult struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Skipped bool   `json:"skipped"` // adapter does not support it
	Message string `json:"message"`
	MS      int64  `json:"ms"`
}

func check(name string, fn func() (string, error)) CheckResult {
	t0 := time.Now()
	msg, err := fn()
	r := CheckResult{Name: name, Message: msg, MS: time.Since(t0).Milliseconds()}
	switch {
	case err == nil:
		r.OK = true
	case errors.Is(err, ErrUnsupported):
		r.Skipped, r.Message = true, "该上游类型不支持"
	default:
		r.Message = err.Error()
	}
	return r
}

// Test runs the five wizard checks against an unsaved or saved site.
func (s *Syncer) Test(ctx context.Context, site Site) []CheckResult {
	site.HTTP = s.HTTP
	ad, err := Get(site.Type)
	if err != nil {
		return []CheckResult{{Name: "站点可达", Message: err.Error()}}
	}
	return []CheckResult{
		check("站点可达", func() (string, error) {
			p, err := ad.Probe(ctx, site)
			return p.Evidence, err
		}),
		check("凭证有效", func() (string, error) { return "", ad.Verify(ctx, site) }),
		check("令牌列表", func() (string, error) {
			ts, err := ad.ListTokens(ctx, site)
			return strconv.Itoa(len(ts)) + " 个令牌", err
		}),
		check("分组倍率", func() (string, error) {
			p, err := ad.Pricing(ctx, site)
			return fmt.Sprintf("%d 个分组，%d 条模型价格", len(p.Groups), len(p.Models)), err
		}),
		check("公告", func() (string, error) {
			a, err := ad.Announcements(ctx, site)
			if a.Content == "" && err == nil {
				return "无公告", nil
			}
			return "有公告", err
		}),
	}
}

// ProbeSite auto-detects the site kind: newapi first, then generic.
// Credentials are not needed. The error lists why each adapter failed.
func (s *Syncer) ProbeSite(ctx context.Context, baseURL string) (ProbeResult, error) {
	var errs []error
	for _, typ := range []string{store.UpstreamNewAPI, store.UpstreamGeneric} {
		ad, err := Get(typ)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		r, err := ad.Probe(ctx, Site{Type: typ, BaseURL: baseURL, HTTP: s.HTTP})
		if err == nil {
			return r, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", typ, err))
	}
	return ProbeResult{}, errors.Join(errs...)
}

// CreateToken creates a token on the upstream, stores it (sealed) and audits it.
func (s *Syncer) CreateToken(ctx context.Context, id uint, in TokenCreate) (store.UpstreamToken, error) {
	u, site, ad, err := s.load(ctx, id)
	if err != nil {
		return store.UpstreamToken{}, err
	}
	rt, err := ad.CreateToken(ctx, site, in)
	if err != nil {
		s.audit(ctx, u, "create_token", "在上游 "+u.Name+" 创建令牌 "+in.Name, err)
		return store.UpstreamToken{}, err
	}
	t := store.UpstreamToken{
		UpstreamID: u.ID, RemoteID: rt.RemoteID, Name: rt.Name, Group: rt.Group, QuotaUSD: rt.QuotaUSD,
		Unlimited: rt.Unlimited, Status: rt.Status, UpdatedAt: time.Now().Unix(),
	}
	if rt.Key != "" {
		if t.Key, err = s.Box.Seal([]byte(rt.Key)); err != nil {
			return store.UpstreamToken{}, err
		}
		t.KeyHint = keyHint(rt.Key)
	}
	err = s.St.DB.WithContext(ctx).Create(&t).Error
	s.audit(ctx, u, "create_token", "在上游 "+u.Name+" 创建令牌 "+in.Name, err)
	return t, err
}

func (s *Syncer) audit(ctx context.Context, u store.Upstream, action, summary string, err error) {
	if s.Audit == nil {
		return
	}
	e := store.AuditLog{Kind: store.AuditUpstream, Target: "upstream:" + strconv.Itoa(int(u.ID)), Action: action, Summary: summary}
	if err != nil {
		e.Result, e.Error = store.ResultFail, err.Error()
	}
	s.Audit.Record(ctx, e)
}
