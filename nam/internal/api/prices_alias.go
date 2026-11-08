package api

import (
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/pricing"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"gorm.io/gorm"
)

// GET /aliases → [{official, aliases[]}] sorted by official name.
func (s *Server) listAliases(c *gin.Context) {
	var as []store.ModelAlias
	if err := s.Store.DB.WithContext(c.Request.Context()).Order("official, alias").Find(&as).Error; err != nil {
		resp.FailFrom(c, err)
		return
	}
	idx := aliasIndex(as)
	names := make([]string, 0, len(idx))
	for k := range idx {
		names = append(names, k)
	}
	sort.Strings(names)
	type group struct {
		Official string   `json:"official"`
		Aliases  []string `json:"aliases"`
	}
	out := make([]group, 0, len(names))
	for _, n := range names {
		out = append(out, group{Official: n, Aliases: idx[n]})
	}
	resp.OK(c, out)
}

// PUT /aliases/:official {aliases:[...]} replaces the alias set of one official model.
// An alias may not be an existing official model name nor belong to another official.
func (s *Server) putAliases(c *gin.Context) {
	official := strings.TrimSpace(c.Param("official"))
	var in struct {
		Aliases []string `json:"aliases"`
	}
	if !resp.BindJSON(c, &in) {
		return
	}
	if official == "" {
		resp.FailFrom(c, resp.Invalid("原厂模型名不能为空"))
		return
	}
	seen := map[string]bool{}
	aliases := make([]string, 0, len(in.Aliases))
	for _, a := range in.Aliases {
		a = strings.TrimSpace(a)
		switch {
		case a == "" || seen[a]:
			continue
		case a == official:
			resp.FailFrom(c, resp.Invalid("别名不能与原厂模型名相同"))
			return
		}
		seen[a] = true
		aliases = append(aliases, a)
	}
	sort.Strings(aliases)
	ctx := c.Request.Context()
	err := s.Store.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, a := range aliases {
			var n int64
			if err := tx.Model(&store.OfficialPrice{}).Where("model = ?", a).Count(&n).Error; err != nil {
				return err
			}
			if n == 0 {
				if err := tx.Model(&store.PriceOverride{}).Where("model = ?", a).Count(&n).Error; err != nil {
					return err
				}
			}
			if n > 0 {
				return resp.Invalid("别名 " + a + " 已是原厂价库中的模型名")
			}
			var other store.ModelAlias
			err := tx.Where("alias = ? AND official <> ?", a, official).Limit(1).Find(&other).Error
			if err != nil {
				return err
			}
			if other.Alias != "" {
				return resp.Conflict("别名 " + a + " 已属于 " + other.Official)
			}
		}
		if err := tx.Where("official = ?", official).Delete(&store.ModelAlias{}).Error; err != nil {
			return err
		}
		for _, a := range aliases {
			if err := tx.Create(&store.ModelAlias{Alias: a, Official: official}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"official": official, "aliases": aliases})
}

// DeriveChannel is one channel's derivation preview.
type DeriveChannel struct {
	ChannelID  int                `json:"channel_id"`
	Name       string             `json:"name"`
	Group      string             `json:"group"`
	Discount   float64            `json:"discount"`        // 当前折扣
	Source     string             `json:"discount_source"` // auto / manual
	Derivation pricing.Derivation `json:"derivation"`
}

// POST /upstreams/:id/derive → per-channel derivation preview (writes nothing).
func (s *Server) deriveUpstream(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	ctx := c.Request.Context()
	if _, err := s.Store.GetUpstream(ctx, id); err != nil {
		resp.FailFrom(c, err)
		return
	}
	inputs, err := s.Channels.PricingInputs(ctx)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	rows, err := s.Channels.List(ctx)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	byID := map[int]int{}
	for i, r := range rows {
		byID[r.ID] = i
	}
	book, err := pricing.LoadBook(ctx, s.Store)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	out := []DeriveChannel{}
	for _, in := range inputs {
		if in.UpstreamID != id {
			continue
		}
		d, err := pricing.DeriveFor(ctx, s.Store, book, in)
		if err != nil {
			resp.FailFrom(c, err)
			return
		}
		dc := DeriveChannel{ChannelID: in.ChannelID, Group: in.Group, Derivation: d}
		if i, ok := byID[in.ChannelID]; ok {
			dc.Name, dc.Discount, dc.Source = rows[i].Name, rows[i].Discount, rows[i].DiscountSource
		}
		out = append(out, dc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ChannelID < out[j].ChannelID })
	resp.OK(c, gin.H{"upstream_id": id, "channels": out})
}
