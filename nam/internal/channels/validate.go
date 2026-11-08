package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/steven52880/nam/internal/store"
)

// vendorIndex answers "which vendor owns this model / group".
type vendorIndex struct {
	byName   map[string]store.Vendor
	byGroup  map[string]store.Vendor
	explicit map[string]string // model -> vendor name (from Vendor.Models)
	patterns []vendorPattern
}

type vendorPattern struct {
	vendor string
	re     *regexp.Regexp
}

func loadVendors(ctx context.Context, st *store.Store) (*vendorIndex, error) {
	var vs []store.Vendor
	if err := st.DB.WithContext(ctx).Order("`order`, id").Find(&vs).Error; err != nil {
		return nil, err
	}
	ix := &vendorIndex{byName: map[string]store.Vendor{}, byGroup: map[string]store.Vendor{}, explicit: map[string]string{}}
	for _, v := range vs {
		ix.byName[v.Name] = v
		ix.byGroup[v.GroupName] = v
		var models []string
		if v.Models != "" {
			_ = json.Unmarshal([]byte(v.Models), &models)
		}
		for _, m := range models {
			ix.explicit[m] = v.Name
		}
		if v.Pattern != "" {
			if re, err := regexp.Compile(v.Pattern); err == nil {
				ix.patterns = append(ix.patterns, vendorPattern{vendor: v.Name, re: re})
			}
		}
	}
	return ix, nil
}

// ownerOf returns the vendor a model is declared for ("" if undeclared).
func (ix *vendorIndex) ownerOf(model string) string {
	if v, ok := ix.explicit[model]; ok {
		return v
	}
	for _, p := range ix.patterns {
		if p.re.MatchString(model) {
			return p.vendor
		}
	}
	return ""
}

// channelTypeFor returns the NewAPI channel type for a vendor (default PassThrough).
func (ix *vendorIndex) channelTypeFor(vendor string) int {
	if v, ok := ix.byName[vendor]; ok && v.ChannelType != 0 {
		return v.ChannelType
	}
	return store.ChannelTypePassThrough
}

// ValidationIssue is one problem found when validating drafts.
type ValidationIssue struct {
	ChannelID int    `json:"channel_id"`
	Name      string `json:"name"`
	Field     string `json:"field"`
	Message   string `json:"message"`
}

// validateRows checks the invariants on the overlaid channel set:
//   - names non-empty and unique
//   - managed channel: group must equal its vendor's group
//   - a model must not be declared for another vendor
//   - a model must not appear in channels of two different vendor groups
//     (auto would pick the first group in AutoGroups and bill the wrong price)
//
// Only rows in `changed` produce issues, so pre-existing NewAPI mess does not block edits
// to unrelated channels.
func validateRows(rows []Row, changed map[int]bool, ix *vendorIndex) []ValidationIssue {
	var issues []ValidationIssue
	names := map[string][]int{}
	modelGroups := map[string]map[string]bool{}
	for _, r := range rows {
		if r.DraftOp == store.DraftDelete {
			continue
		}
		names[r.Name] = append(names[r.Name], r.ID)
		if _, isVendorGroup := ix.byGroup[r.Group]; !isVendorGroup {
			continue
		}
		for _, m := range r.Models {
			if modelGroups[m] == nil {
				modelGroups[m] = map[string]bool{}
			}
			modelGroups[m][r.Group] = true
		}
	}
	for _, r := range rows {
		if !changed[r.ID] || r.DraftOp == store.DraftDelete {
			continue
		}
		add := func(field, msg string) {
			issues = append(issues, ValidationIssue{ChannelID: r.ID, Name: r.Name, Field: field, Message: msg})
		}
		if strings.TrimSpace(r.Name) == "" {
			add("name", "名称不能为空")
		} else if len(names[r.Name]) > 1 {
			add("name", "名称重复："+r.Name)
		}
		if r.Weight > 1<<31 {
			add("weight", "权重过大")
		}
		if r.Vendor != "" {
			v, ok := ix.byName[r.Vendor]
			switch {
			case !ok:
				add("vendor", "未知厂商 "+r.Vendor)
			case r.Group != v.GroupName:
				add("group", fmt.Sprintf("渠道分组必须是厂商 %s 的分组 %s", v.Name, v.GroupName))
			}
		}
		if r.Discount <= 0 && r.Managed {
			add("discount", "上游分组折扣必须大于 0")
		}
		for _, m := range r.Models {
			if owner := ix.ownerOf(m); owner != "" && r.Vendor != "" && owner != r.Vendor {
				add("models", fmt.Sprintf("模型 %s 属于厂商 %s，不能放进 %s 的渠道", m, owner, r.Vendor))
			}
			if gs := modelGroups[m]; len(gs) > 1 {
				add("models", fmt.Sprintf("模型 %s 同时出现在分组 %s，auto 会按顺序选错分组", m, joinKeys(gs)))
			}
		}
	}
	return issues
}

func joinKeys(m map[string]bool) string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return strings.Join(ks, "、")
}
