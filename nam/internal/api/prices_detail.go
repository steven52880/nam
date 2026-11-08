package api

import (
	"errors"
	"math"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/pricing"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"gorm.io/gorm"
)

const jobWaitTimeout = 10 * time.Minute

func nowUnix() int64 { return time.Now().Unix() }

// ChannelUse is one channel serving the model, with its cost for it.
type ChannelUse struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Vendor      string   `json:"vendor"`
	Discount    float64  `json:"discount"` // 实际使用的折扣（模型级覆盖优先）
	Override    *float64 `json:"override"`
	CostInPerM  *float64 `json:"cost_in_per_m"`
	CostOutPerM *float64 `json:"cost_out_per_m"`
	CostPerReq  *float64 `json:"cost_per_request"`
}

// GET /prices/:model
func (s *Server) priceDetail(c *gin.Context) {
	ctx := c.Request.Context()
	model := strings.TrimSpace(c.Param("model"))
	book, err := pricing.LoadBook(ctx, s.Store)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	official := book.Canonical(model)
	out := gin.H{"model": model, "official_name": official}
	if p, ok := book.Resolve(model); ok {
		out["official"] = p
	} else {
		out["official"] = nil
	}
	var bookRow store.OfficialPrice
	if err := s.Store.DB.WithContext(ctx).First(&bookRow, "model = ?", official).Error; err == nil {
		out["book"] = bookRow
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		resp.FailFrom(c, err)
		return
	} else {
		out["book"] = nil
	}
	var ov store.PriceOverride
	if err := s.Store.DB.WithContext(ctx).First(&ov, "model = ?", official).Error; err == nil {
		out["override"] = ov
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		resp.FailFrom(c, err)
		return
	} else {
		out["override"] = nil
	}
	var as []store.ModelAlias
	if err := s.Store.DB.WithContext(ctx).Where("official = ?", official).Order("alias").Find(&as).Error; err != nil {
		resp.FailFrom(c, err)
		return
	}
	aliases := make([]string, 0, len(as))
	for _, a := range as {
		aliases = append(aliases, a.Alias)
	}
	out["aliases"] = aliases
	uses, err := s.channelUses(c, book, model)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	out["channels"] = uses
	resp.OK(c, out)
}

func (s *Server) channelUses(c *gin.Context, book *pricing.Book, model string) ([]ChannelUse, error) {
	ctx := c.Request.Context()
	rows, err := s.Channels.List(ctx)
	if err != nil {
		return nil, err
	}
	var exts []store.ChannelExt
	if err := s.Store.DB.WithContext(ctx).Find(&exts).Error; err != nil {
		return nil, err
	}
	extByID := map[int]store.ChannelExt{}
	for _, e := range exts {
		extByID[e.ChannelID] = e
	}
	price, priced := book.Resolve(model)
	want := book.Canonical(model)
	uses := []ChannelUse{}
	for _, r := range rows {
		var served string
		for _, m := range r.Models {
			if m == model || book.Canonical(m) == want {
				served = m
				break
			}
		}
		if served == "" {
			continue
		}
		ov := pricing.ModelDiscounts(extByID[r.ID].ModelDiscount)
		u := ChannelUse{ID: r.ID, Name: r.Name, Vendor: r.Vendor, Discount: pricing.EffectiveDiscount(r.Discount, ov, served)}
		if v, ok := ov[served]; ok {
			u.Override = &v
		}
		if priced && r.CurrencyRate != nil {
			if price.IsPerRequest() {
				v := pricing.ChannelCNY(*price.PerRequest, u.Discount, *r.CurrencyRate)
				u.CostPerReq = &v
			} else {
				u.CostInPerM, u.CostOutPerM = pricing.PerMillionCNY(price, u.Discount, *r.CurrencyRate)
			}
		}
		uses = append(uses, u)
	}
	return uses, nil
}

// OverrideInput is the PUT /prices/:model/override body.
type OverrideInput struct {
	PerRequestMode bool     `json:"per_request_mode"`
	Input          *float64 `json:"input"`
	Output         *float64 `json:"output"`
	CacheRead      *float64 `json:"cache_read"`
	CacheWrite5m   *float64 `json:"cache_write_5m"`
	CacheWrite1h   *float64 `json:"cache_write_1h"`
	PerRequest     *float64 `json:"per_request"`
	Remark         string   `json:"remark"`
}

func (in OverrideInput) validate() error {
	for _, v := range []*float64{in.Input, in.Output, in.CacheRead, in.CacheWrite5m, in.CacheWrite1h, in.PerRequest} {
		if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0) {
			return resp.Invalid("价格必须是不小于 0 的有限数")
		}
	}
	if in.PerRequestMode {
		if in.PerRequest == nil || *in.PerRequest <= 0 {
			return resp.Invalid("按次计费必须填写大于 0 的单次价格")
		}
		return nil
	}
	if in.Input == nil || *in.Input <= 0 {
		return resp.Invalid("按 token 计费必须填写大于 0 的输入价")
	}
	return nil
}

// PUT /prices/:model/override
func (s *Server) putOverride(c *gin.Context) {
	model := strings.TrimSpace(c.Param("model"))
	var in OverrideInput
	if !resp.BindJSON(c, &in) {
		return
	}
	if model == "" {
		resp.FailFrom(c, resp.Invalid("模型名不能为空"))
		return
	}
	if err := in.validate(); err != nil {
		resp.FailFrom(c, err)
		return
	}
	row := store.PriceOverride{Model: model, PerRequest: in.PerRequestMode, CacheRead: in.CacheRead, CacheWrite5m: in.CacheWrite5m,
		CacheWrite1h: in.CacheWrite1h, Remark: in.Remark, UpdatedAt: nowUnix()}
	if in.PerRequestMode {
		row.PerRequestUSD = in.PerRequest
	} else {
		row.Input, row.Output = in.Input, in.Output
	}
	if err := s.Store.DB.WithContext(c.Request.Context()).Save(&row).Error; err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, row)
}

// DELETE /prices/:model/override
func (s *Server) deleteOverride(c *gin.Context) {
	model := strings.TrimSpace(c.Param("model"))
	res := s.Store.DB.WithContext(c.Request.Context()).Where("model = ?", model).Delete(&store.PriceOverride{})
	if res.Error != nil {
		resp.FailFrom(c, res.Error)
		return
	}
	resp.OK(c, nil)
}
