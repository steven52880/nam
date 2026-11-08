package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"gorm.io/gorm"
)

// CreateSpec is one channel to create (as a draft).
type CreateSpec struct {
	Fields Fields    `json:"fields"`
	Ext    ExtFields `json:"ext"`
}

// AddCreateDrafts validates and stores create drafts. Returns the temp ids, or issues.
func (s *Service) AddCreateDrafts(ctx context.Context, specs []CreateSpec) ([]int, []ValidationIssue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.load(ctx)
	if err != nil {
		return nil, nil, err
	}
	var ids []int
	var drafts []store.ChannelDraft
	changed := map[int]bool{}
	err = s.St.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		next, err := nextTempID(tx)
		if err != nil {
			return err
		}
		for _, sp := range specs {
			if sp.Ext.TokenID == nil || sp.Ext.UpstreamID == nil {
				return resp.Invalid("新建渠道必须指定上游和令牌")
			}
			if sp.Ext.DiscountSource == nil {
				src := store.DiscountAuto
				sp.Ext.DiscountSource = &src
			}
			raw, _ := json.Marshal(draftBody{Fields: sp.Fields, Ext: sp.Ext})
			now := time.Now().Unix()
			d := store.ChannelDraft{ChannelID: next, Op: store.DraftCreate, Diff: string(raw), CreatedAt: now, UpdatedAt: now}
			st.drafts[next] = d
			changed[next] = true
			ids = append(ids, next)
			drafts = append(drafts, d)
			next--
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	if issues := validateRows(st.rows(), changed, st.vendors); len(issues) > 0 {
		return nil, issues, nil
	}
	return ids, nil, s.St.DB.WithContext(ctx).Create(&drafts).Error
}

// BatchRequest is the batch-create wizard input: one upstream, several tokens, a model
// list that is split by vendor (one channel per token × vendor).
type BatchRequest struct {
	UpstreamID uint     `json:"upstream_id"`
	TokenIDs   []uint   `json:"token_ids"`
	Models     []string `json:"models"`
	Priority   int64    `json:"priority"`
	Weight     uint     `json:"weight"`
	Tag        string   `json:"tag"`
	// TestModel per vendor; empty → first model of that vendor in the request order.
	// (M3 replaces the fallback with "cheapest model by official price".)
	TestModel map[string]string `json:"test_model"`
	// Discount per token id (string key); empty → derived from upstream group ratio, else 1.
	Discount map[string]float64 `json:"discount"`
}

// BatchPreviewItem is one channel the wizard will create.
type BatchPreviewItem struct {
	CreateSpec
	Name        string `json:"name"`
	TokenName   string `json:"token_name"`
	Vendor      string `json:"vendor"`
	Group       string `json:"group"`
	ChannelType int    `json:"channel_type"`
	Duplicate   bool   `json:"duplicate"` // name already exists in NewAPI or drafts
}

// BatchPreview expands a batch request into channel specs (no writes).
// Models without a vendor are returned in `unassigned` and skipped.
func (s *Service) BatchPreview(ctx context.Context, req BatchRequest) (items []BatchPreviewItem, unassigned []string, err error) {
	st, err := s.load(ctx)
	if err != nil {
		return nil, nil, err
	}
	up, ok := st.ups[req.UpstreamID]
	if !ok {
		return nil, nil, resp.Invalid("上游不存在")
	}
	byVendor := map[string][]string{}
	var vendorOrder []string
	for _, m := range dedupe(req.Models) {
		v := st.vendors.ownerOf(m)
		if v == "" {
			unassigned = append(unassigned, m)
			continue
		}
		if _, seen := byVendor[v]; !seen {
			vendorOrder = append(vendorOrder, v)
		}
		byVendor[v] = append(byVendor[v], m)
	}
	existing := map[string]bool{}
	for _, r := range st.rows() {
		existing[r.Name] = true
	}
	for _, tid := range req.TokenIDs {
		tok, ok := st.tokens[tid]
		if !ok || tok.UpstreamID != up.ID {
			return nil, nil, resp.Invalid(fmt.Sprintf("令牌 %d 不属于该上游", tid))
		}
		disc := 1.0
		if d, ok := req.Discount[fmt.Sprint(tid)]; ok && d > 0 {
			disc = d
		}
		for _, vname := range vendorOrder {
			v := st.vendors.byName[vname]
			models := byVendor[vname]
			name := ChannelName(up.Name, tok.Name, vname)
			test := req.TestModel[vname]
			if test == "" {
				test = models[0]
			}
			typ := st.vendors.channelTypeFor(vname)
			base := up.BaseURL
			upID, tokID, vn, group := up.ID, tok.ID, vname, v.GroupName
			it := BatchPreviewItem{
				CreateSpec: CreateSpec{
					Fields: Fields{Name: &name, Type: &typ, BaseURL: &base, Models: &models, Group: &group,
						Priority: &req.Priority, Weight: &req.Weight, Tag: &req.Tag, TestModel: &test, Status: ptrInt(1)},
					Ext: ExtFields{UpstreamID: &upID, TokenID: &tokID, Vendor: &vn, Discount: &disc},
				},
				Name: name, TokenName: tok.Name, Vendor: vname, Group: group, ChannelType: typ, Duplicate: existing[name],
			}
			existing[name] = true
			items = append(items, it)
		}
	}
	sort.Strings(unassigned)
	return items, unassigned, nil
}

// ChannelName builds nam-<upstream>-<token>-<vendor>, trimmed to NewAPI-friendly length.
func ChannelName(upstream, token, vendor string) string {
	clean := func(s string) string {
		s = strings.TrimSpace(s)
		s = strings.Map(func(r rune) rune {
			if r == ' ' || r == '/' || r == '\\' {
				return '_'
			}
			return r
		}, s)
		return s
	}
	n := "nam-" + clean(upstream) + "-" + clean(token) + "-" + clean(vendor)
	if r := []rune(n); len(r) > 120 {
		n = string(r[:120])
	}
	return n
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func ptrInt(v int) *int { return &v }
