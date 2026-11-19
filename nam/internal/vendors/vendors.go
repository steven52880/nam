// Package vendors manages vendors: one vendor = one NewAPI group plus the models it owns.
// Writing GroupRatio/AutoGroups/UserUsableGroups to NewAPI is M5 and is not done here.
package vendors

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
	"gorm.io/gorm"
)

// maxNameLen mirrors NewAPI's group column (varchar(64)).
const maxNameLen = 64

var groupRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Input is the create/update body.
type Input struct {
	Name        string   `json:"name"`
	GroupName   string   `json:"group_name"`
	Models      []string `json:"models"`
	Prefixes    []string `json:"prefixes"`
	ChannelType int      `json:"channel_type"` // 0 → 58
}

// View is a vendor in API responses.
type View struct {
	store.Vendor
	ModelList    []string `json:"model_list"`
	ChannelCount int      `json:"channel_count"`
}

// Service is the vendor CRUD layer. Channels is read to count and protect vendors: a
// vendor is in use by the NewAPI channels whose groups imply it (channel_ext no longer
// stores a vendor). A nil Channels counts nothing.
type Service struct {
	St       *store.Store
	Channels ports.ChannelStore
}

// channelCounts counts, per vendor name, the NewAPI channels whose groups imply it
// (first vendor group in vendor order wins, see VendorOfGroups).
func (s *Service) channelCounts(ctx context.Context, vs []store.Vendor) (map[string]int, error) {
	out := map[string]int{}
	if s.Channels == nil {
		return out, nil
	}
	chs, err := s.Channels.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 NewAPI 渠道失败：%w", err)
	}
	pairs := make([]VendorGroup, 0, len(vs))
	for _, v := range vs {
		pairs = append(pairs, VendorGroup{Name: v.Name, Group: v.GroupName})
	}
	for _, c := range chs {
		if name := VendorOfGroups(pairs, c.Group); name != "" {
			out[name]++
		}
	}
	return out, nil
}

func parseModels(raw string) []string {
	var ms []string
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &ms)
	}
	if ms == nil {
		ms = []string{}
	}
	return ms
}

func normalize(in Input) (Input, error) {
	in.Name, in.GroupName = strings.TrimSpace(in.Name), strings.TrimSpace(in.GroupName)
	switch {
	case in.Name == "" || len([]rune(in.Name)) > maxNameLen:
		return in, resp.Invalid("厂商名称不能为空，且不超过 64 个字符")
	case in.GroupName == "" || len(in.GroupName) > maxNameLen:
		return in, resp.Invalid("分组名不能为空，且不超过 64 个字符")
	case !groupRe.MatchString(in.GroupName):
		return in, resp.Invalid("分组名只能包含字母、数字、下划线和连字符")
	}
	if in.ChannelType == 0 {
		in.ChannelType = store.ChannelTypePassThrough
	}
	if in.ChannelType != store.ChannelTypePassThrough && in.ChannelType != store.ChannelTypeGemini {
		return in, resp.Invalid(fmt.Sprintf("渠道类型只能是 %d 或 %d", store.ChannelTypePassThrough, store.ChannelTypeGemini))
	}
	prefixSeen := map[string]bool{}
	prefixes := make([]string, 0, len(in.Prefixes))
	for _, p := range in.Prefixes {
		p = strings.TrimSpace(p)
		if p == "" {
			return in, resp.Invalid("模型名前缀不能为空")
		}
		if key := strings.ToLower(p); !prefixSeen[key] {
			prefixSeen[key] = true
			prefixes = append(prefixes, p)
		}
	}
	in.Prefixes = prefixes
	seen := map[string]bool{}
	models := make([]string, 0, len(in.Models))
	for _, m := range in.Models {
		if m = strings.TrimSpace(m); m != "" && !seen[m] {
			seen[m] = true
			models = append(models, m)
		}
	}
	in.Models = models
	return in, nil
}

// List returns vendors ordered by Order, with channel counts.
func (s *Service) List(ctx context.Context) ([]View, error) {
	var vs []store.Vendor
	if err := s.St.DB.WithContext(ctx).Order("`order`, id").Find(&vs).Error; err != nil {
		return nil, err
	}
	byName, err := s.channelCounts(ctx, vs)
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, len(vs))
	for _, v := range vs {
		if v.Prefixes == nil {
			v.Prefixes = []string{}
		}
		out = append(out, View{Vendor: v, ModelList: parseModels(v.Models), ChannelCount: byName[v.Name]})
	}
	return out, nil
}

// channelsUsing counts the channels whose groups imply vendor `name`.
func (s *Service) channelsUsing(ctx context.Context, name string) (int, error) {
	var vs []store.Vendor
	if err := s.St.DB.WithContext(ctx).Order("`order`, id").Find(&vs).Error; err != nil {
		return 0, err
	}
	counts, err := s.channelCounts(ctx, vs)
	return counts[name], err
}

// checkConflicts rejects duplicate names/groups and models that another vendor already owns.
func checkConflicts(tx *gorm.DB, self uint, in Input) error {
	var others []store.Vendor
	if err := tx.Where("id <> ?", self).Find(&others).Error; err != nil {
		return err
	}
	owner := map[string]string{}
	prefixOwner := map[string]string{} // lower-cased prefix -> vendor name
	for _, o := range others {
		if o.Name == in.Name {
			return resp.Conflict("厂商名称已存在：" + in.Name)
		}
		if o.GroupName == in.GroupName {
			return resp.Conflict(fmt.Sprintf("分组 %s 已被厂商 %s 使用", in.GroupName, o.Name))
		}
		for _, m := range parseModels(o.Models) {
			owner[m] = o.Name
		}
		for _, p := range o.Prefixes {
			prefixOwner[strings.ToLower(strings.TrimSpace(p))] = o.Name
		}
	}
	for _, p := range in.Prefixes {
		if o, taken := prefixOwner[strings.ToLower(p)]; taken {
			return resp.Conflict(fmt.Sprintf("前缀 %s 已被厂商 %s 使用，一个前缀只能属于一个厂商", p, o))
		}
	}
	for _, m := range in.Models {
		if o, taken := owner[m]; taken {
			return resp.Conflict(fmt.Sprintf("模型 %s 同时属于厂商 %s 和 %s，一个模型只能属于一个厂商", m, o, in.Name))
		}
	}
	return nil
}

func apply(v *store.Vendor, in Input) {
	raw, _ := json.Marshal(in.Models)
	v.Name, v.GroupName, v.Models, v.Prefixes, v.ChannelType = in.Name, in.GroupName, string(raw), in.Prefixes, in.ChannelType
}

// Create adds a vendor at the end of the order.
func (s *Service) Create(ctx context.Context, in Input) (store.Vendor, error) {
	in, err := normalize(in)
	if err != nil {
		return store.Vendor{}, err
	}
	var v store.Vendor
	err = s.St.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := checkConflicts(tx, 0, in); err != nil {
			return err
		}
		var maxOrder *int
		if err := tx.Model(&store.Vendor{}).Select("MAX(`order`)").Scan(&maxOrder).Error; err != nil {
			return err
		}
		if maxOrder != nil {
			v.Order = *maxOrder + 1
		}
		apply(&v, in)
		return tx.Create(&v).Error
	})
	return v, err
}

// UpdateTx edits a vendor inside the caller's transaction and returns the vendor before
// and after. Renaming a vendor or its group while channels use it is allowed: the caller
// (channels.Service.UpdateVendor) migrates channel_ext and creates the group rename drafts.
func UpdateTx(tx *gorm.DB, id uint, in Input) (old, cur store.Vendor, err error) {
	in, err = normalize(in)
	if err != nil {
		return old, cur, err
	}
	if err = tx.First(&old, id).Error; err != nil {
		return old, cur, mapNotFound(err)
	}
	if err = checkConflicts(tx, id, in); err != nil {
		return old, cur, err
	}
	cur = old
	apply(&cur, in)
	if err = tx.Save(&cur).Error; err != nil {
		return old, cur, err
	}
	return old, cur, nil
}

// Delete removes a vendor unless a channel uses it (a NewAPI channel carrying its group).
func (s *Service) Delete(ctx context.Context, id uint) error {
	var v store.Vendor
	if err := s.St.DB.WithContext(ctx).First(&v, id).Error; err != nil {
		return mapNotFound(err)
	}
	// NewAPI is read outside the transaction: the store allows a single connection.
	n, err := s.channelsUsing(ctx, v.Name)
	if err != nil {
		return err
	}
	if n > 0 {
		return resp.Conflict(fmt.Sprintf("厂商 %s 仍有 %d 个渠道在用，不能删除", v.Name, n))
	}
	return s.St.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&v, id).Error; err != nil {
			return mapNotFound(err)
		}
		if err := tx.Where("vendor_id = ?", id).Delete(&store.VendorPricing{}).Error; err != nil {
			return err
		}
		return tx.Delete(&store.Vendor{}, id).Error
	})
}

// Reorder sets Order to each id's position. ids must list every vendor exactly once.
func (s *Service) Reorder(ctx context.Context, ids []uint) error {
	return s.St.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var total int64
		if err := tx.Model(&store.Vendor{}).Count(&total).Error; err != nil {
			return err
		}
		seen := map[uint]bool{}
		for _, id := range ids {
			seen[id] = true
		}
		if int64(len(ids)) != total || len(seen) != len(ids) {
			return resp.Invalid("排序必须包含全部厂商且不重复")
		}
		for i, id := range ids {
			res := tx.Model(&store.Vendor{}).Where("id = ?", id).Update("order", i)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return resp.Invalid(fmt.Sprintf("厂商 %d 不存在", id))
			}
		}
		return nil
	})
}

func mapNotFound(err error) error {
	if err == gorm.ErrRecordNotFound {
		return store.ErrNotFound
	}
	return err
}
