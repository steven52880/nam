package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/steven52880/nam/internal/pricing"
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

// BatchRequest is the batch-create wizard input: one upstream, several tokens, and a model
// list per token that is split by vendor (one channel per token × vendor).
type BatchRequest struct {
	UpstreamID uint   `json:"upstream_id"`
	TokenIDs   []uint `json:"token_ids"`
	// TokenModels per token id (string key); empty → the token's stored Models.
	TokenModels map[string][]string `json:"token_models"`
	Priority    int64               `json:"priority"`
	Weight      uint                `json:"weight"`
	Tag         string              `json:"tag"`
	// TestModel per vendor; empty → cheapest model of that vendor by official price
	// (first model when none is priced).
	TestModel map[string]string `json:"test_model"`
	// Discount per token id (string key, manual); empty → derived from upstream prices, else 1.
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

// BatchPreviewResult is the wizard preview: the channels to create plus what was skipped.
type BatchPreviewResult struct {
	Items             []BatchPreviewItem  `json:"items"`
	Warnings          []string            `json:"warnings"`
	Unassigned        []string            `json:"unassigned"`          // sorted union over all tokens
	UnassignedByToken map[string][]string `json:"unassigned_by_token"` // token id → models without a vendor
}

// tokenVendorSplit groups one token's models by owning vendor (first-seen vendor order).
func tokenVendorSplit(st *state, models []string) (byVendor map[string][]string, order, unassigned []string) {
	byVendor = map[string][]string{}
	for _, m := range models {
		v := st.vendors.ownerOf(m)
		if v == "" {
			unassigned = append(unassigned, m)
			continue
		}
		if _, seen := byVendor[v]; !seen {
			order = append(order, v)
		}
		byVendor[v] = append(byVendor[v], m)
	}
	return byVendor, order, unassigned
}

// BatchPreview expands a batch request into channel specs (no writes). Each token's
// requested models are split by vendor, one item per (token, vendor). Models without
// a vendor are reported and skipped.
func (s *Service) BatchPreview(ctx context.Context, req BatchRequest) (BatchPreviewResult, error) {
	out := BatchPreviewResult{Items: []BatchPreviewItem{}, Warnings: []string{}, Unassigned: []string{},
		UnassignedByToken: map[string][]string{}}
	st, err := s.load(ctx)
	if err != nil {
		return out, err
	}
	up, ok := st.ups[req.UpstreamID]
	if !ok {
		return out, resp.Invalid("上游不存在")
	}
	existing := map[string]bool{}
	for _, r := range st.rows() {
		existing[r.Name] = true
	}
	book, err := pricing.LoadBook(ctx, s.St)
	if err != nil {
		return out, err
	}
	unionSet := map[string]bool{}
	for _, tid := range req.TokenIDs {
		tok, ok := st.tokens[tid]
		if !ok || tok.UpstreamID != up.ID {
			return out, resp.Invalid(fmt.Sprintf("令牌 %d 不属于该上游", tid))
		}
		tokModels := dedupe(req.TokenModels[fmt.Sprint(tid)])
		if len(tokModels) == 0 {
			return out, resp.Invalid(fmt.Sprintf("令牌 %s 没有选择模型", tok.Name))
		}
		byVendor, vendorOrder, miss := tokenVendorSplit(st, tokModels)
		if len(miss) > 0 {
			sort.Strings(miss)
			out.UnassignedByToken[fmt.Sprint(tid)] = miss
			for _, m := range miss {
				unionSet[m] = true
			}
		}
		if len(vendorOrder) > 1 {
			out.Warnings = append(out.Warnings, fmt.Sprintf("令牌 %s 的模型属于 %d 个厂商，将拆成 %d 个渠道", tok.Name, len(vendorOrder), len(vendorOrder)))
		}
		manual, hasManual := req.Discount[fmt.Sprint(tid)]
		hasManual = hasManual && manual > 0
		for _, vname := range vendorOrder {
			v := st.vendors.byName[vname]
			models := byVendor[vname]
			name := ChannelName(up.Name, tok.Name, vname)
			test := req.TestModel[vname]
			if test == "" {
				test = testModelFor(book, models)
			}
			// 上游分组折扣: manual from the wizard, else derived from upstream prices, else 1.
			disc, src := 1.0, store.DiscountAuto
			if hasManual {
				disc, src = manual, store.DiscountManual
			} else if d, err := pricing.DeriveFor(ctx, s.St, book, pricing.ChannelInput{UpstreamID: up.ID, Group: tok.Group, Models: models}); err == nil && d.Median != nil {
				disc = pricing.RoundDiscount(*d.Median)
			}
			typ := st.vendors.channelTypeFor(vname)
			base := up.BaseURL
			upID, tokID, vn, group := up.ID, tok.ID, vname, v.GroupName
			it := BatchPreviewItem{
				CreateSpec: CreateSpec{
					Fields: Fields{Name: &name, Type: &typ, BaseURL: &base, Models: &models, Group: &group,
						Priority: &req.Priority, Weight: &req.Weight, Tag: &req.Tag, TestModel: &test, Status: ptrInt(1)},
					Ext: ExtFields{UpstreamID: &upID, TokenID: &tokID, Vendor: &vn, Discount: &disc, DiscountSource: &src},
				},
				Name: name, TokenName: tok.Name, Vendor: vname, Group: group, ChannelType: typ, Duplicate: existing[name],
			}
			existing[name] = true
			out.Items = append(out.Items, it)
		}
	}
	for m := range unionSet {
		out.Unassigned = append(out.Unassigned, m)
	}
	sort.Strings(out.Unassigned)
	return out, nil
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
