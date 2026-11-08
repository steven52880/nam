package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/pricing"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"gorm.io/gorm"
)

func (s *Server) registerChannelPricingRoutes(p *gin.RouterGroup) {
	p.GET("/channels/:id/models", s.channelModels)
	p.PUT("/channels/:id/model-discount", s.putModelDiscount)
	p.PUT("/channels/:id/cache-override", s.putCacheOverride)
}

// GET /channels/:id/models: per-model price / discount / cost breakdown (ui-features §4.4).
func (s *Server) channelModels(c *gin.Context) {
	id, okID := parseChannelID(c)
	if !okID {
		return
	}
	d, err := s.Channels.Models(c.Request.Context(), id)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, d)
}

func finiteNonNeg(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 }

// editExt applies mutate to channel_ext of a managed channel inside one transaction and
// records an audit row. mutate returns the before/after strings for the audit entry.
func (s *Server) editExt(c *gin.Context, id int, action, summary string, mutate func(*store.ChannelExt) (before, after string)) (store.ChannelExt, bool) {
	ctx := c.Request.Context()
	var ext store.ChannelExt
	var before, after string
	err := s.Store.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&ext, "channel_id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return resp.Invalid("渠道未被 nam 管理，先纳入管理或通过 nam 创建")
			}
			return err
		}
		before, after = mutate(&ext)
		return tx.Model(&store.ChannelExt{}).Where("channel_id = ?", id).
			Updates(map[string]any{"model_discount": ext.ModelDiscount, "cache_override": ext.CacheOverride, "updated_at": nowUnix()}).Error
	})
	if err != nil {
		resp.FailFrom(c, err)
		return ext, false
	}
	s.Audit.Record(ctx, store.AuditLog{Kind: store.AuditChannel, Target: "channel:" + strconv.Itoa(id), Action: action,
		Summary: summary, Before: before, After: after})
	return ext, true
}

// PUT /channels/:id/model-discount {model, discount|null}
func (s *Server) putModelDiscount(c *gin.Context) {
	id, okID := parseChannelID(c)
	if !okID {
		return
	}
	var in struct {
		Model    string   `json:"model"`
		Discount *float64 `json:"discount"`
	}
	if !resp.BindJSON(c, &in) {
		return
	}
	model := strings.TrimSpace(in.Model)
	switch {
	case model == "":
		resp.FailFrom(c, resp.Invalid("model 不能为空"))
		return
	case in.Discount != nil && (!finiteNonNeg(*in.Discount) || *in.Discount == 0):
		resp.FailFrom(c, resp.Invalid("模型级折扣必须大于 0；清除覆盖请传 null"))
		return
	}
	ext, ok := s.editExt(c, id, "model_discount", fmt.Sprintf("修改渠道 %d 的模型级折扣 %s", id, model), func(e *store.ChannelExt) (string, string) {
		before := e.ModelDiscount
		m := pricing.ModelDiscounts(e.ModelDiscount)
		if in.Discount == nil {
			delete(m, model)
		} else {
			m[model] = *in.Discount
		}
		e.ModelDiscount = ""
		if len(m) > 0 {
			b, _ := json.Marshal(m)
			e.ModelDiscount = string(b)
		}
		return before, e.ModelDiscount
	})
	if ok {
		resp.OK(c, pricing.ModelDiscounts(ext.ModelDiscount))
	}
}

// PUT /channels/:id/cache-override {read?, write?}: both null/absent clears the override.
func (s *Server) putCacheOverride(c *gin.Context) {
	id, okID := parseChannelID(c)
	if !okID {
		return
	}
	var in pricing.CacheRatio
	if !resp.BindJSON(c, &in) {
		return
	}
	for _, v := range []*float64{in.Read, in.Write} {
		if v != nil && !finiteNonNeg(*v) {
			resp.FailFrom(c, resp.Invalid("缓存比例必须是不小于 0 的有限数"))
			return
		}
	}
	ext, ok := s.editExt(c, id, "cache_override", fmt.Sprintf("修改渠道 %d 的缓存价覆盖", id), func(e *store.ChannelExt) (string, string) {
		before := e.CacheOverride
		e.CacheOverride = ""
		if in.Read != nil || in.Write != nil {
			b, _ := json.Marshal(in)
			e.CacheOverride = string(b)
		}
		return before, e.CacheOverride
	})
	if ok {
		resp.OK(c, pricing.ParseCacheRatio(ext.CacheOverride))
	}
}
