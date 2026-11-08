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
	Pattern     string   `json:"pattern"`
	ChannelType int      `json:"channel_type"` // 0 → 58
}

// View is a vendor in API responses.
type View struct {
	store.Vendor
	ModelList    []string `json:"model_list"`
	ChannelCount int      `json:"channel_count"`
}

// Service is the vendor CRUD layer.
type Service struct{ St *store.Store }

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
	if in.Pattern != "" {
		if _, err := regexp.Compile(in.Pattern); err != nil {
			return in, resp.Invalid("模型匹配正则不合法：" + err.Error())
		}
	}
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
	var counts []struct {
		Vendor string
		N      int
	}
	if err := s.St.DB.WithContext(ctx).Model(&store.ChannelExt{}).Select("vendor, count(*) as n").Group("vendor").Scan(&counts).Error; err != nil {
		return nil, err
	}
	byName := map[string]int{}
	for _, c := range counts {
		byName[c.Vendor] = c.N
	}
	out := make([]View, 0, len(vs))
	for _, v := range vs {
		out = append(out, View{Vendor: v, ModelList: parseModels(v.Models), ChannelCount: byName[v.Name]})
	}
	return out, nil
}

func (s *Service) channelsUsing(tx *gorm.DB, name string) (int64, error) {
	var n int64
	err := tx.Model(&store.ChannelExt{}).Where("vendor = ?", name).Count(&n).Error
	return n, err
}

// checkConflicts rejects duplicate names/groups and models that another vendor already owns.
func checkConflicts(tx *gorm.DB, self uint, in Input) error {
	var others []store.Vendor
	if err := tx.Where("id <> ?", self).Find(&others).Error; err != nil {
		return err
	}
	owner := map[string]string{}
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
	v.Name, v.GroupName, v.Models, v.Pattern, v.ChannelType = in.Name, in.GroupName, string(raw), in.Pattern, in.ChannelType
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

// Update edits a vendor. Name and group cannot change while channels use the vendor.
func (s *Service) Update(ctx context.Context, id uint, in Input) (store.Vendor, error) {
	in, err := normalize(in)
	if err != nil {
		return store.Vendor{}, err
	}
	var v store.Vendor
	err = s.St.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&v, id).Error; err != nil {
			return mapNotFound(err)
		}
		if err := checkConflicts(tx, id, in); err != nil {
			return err
		}
		if in.Name != v.Name || in.GroupName != v.GroupName {
			n, err := s.channelsUsing(tx, v.Name)
			if err != nil {
				return err
			}
			if n > 0 {
				return resp.Conflict(fmt.Sprintf("厂商 %s 仍有 %d 个渠道在用，不能修改名称或分组", v.Name, n))
			}
		}
		apply(&v, in)
		return tx.Save(&v).Error
	})
	return v, err
}

// Delete removes a vendor unless a channel uses it.
func (s *Service) Delete(ctx context.Context, id uint) error {
	return s.St.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var v store.Vendor
		if err := tx.First(&v, id).Error; err != nil {
			return mapNotFound(err)
		}
		n, err := s.channelsUsing(tx, v.Name)
		if err != nil {
			return err
		}
		if n > 0 {
			return resp.Conflict(fmt.Sprintf("厂商 %s 仍有 %d 个渠道在用，不能删除", v.Name, n))
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
