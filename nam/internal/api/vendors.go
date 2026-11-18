package api

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/internal/vendors"
)

func (s *Server) registerVendorRoutes(p *gin.RouterGroup) {
	p.GET("/vendors", s.listVendors)
	p.POST("/vendors", s.createVendor)
	p.POST("/vendors/match", s.matchVendors)
	p.PUT("/vendors/order", s.reorderVendors) // registered before /:id so "order" is not parsed as an id
	p.PUT("/vendors/:id", s.updateVendor)
	p.DELETE("/vendors/:id", s.deleteVendor)
}

func (s *Server) recordVendor(c *gin.Context, action, summary string) {
	s.Audit.Record(c.Request.Context(), store.AuditLog{Kind: store.AuditSetting, Target: "nam:vendor", Action: action, Summary: summary})
}

func (s *Server) listVendors(c *gin.Context) {
	vs, err := s.Vendors.List(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, vs)
}

// POST /vendors/match {models:[...]}: which vendor owns each model, and how it matched.
func (s *Server) matchVendors(c *gin.Context) {
	var in struct {
		Models []string `json:"models"`
	}
	if !resp.BindJSON(c, &in) {
		return
	}
	items, err := s.Vendors.Match(c.Request.Context(), in.Models)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"items": items})
}

func (s *Server) createVendor(c *gin.Context) {
	var in vendors.Input
	if !resp.BindJSON(c, &in) {
		return
	}
	v, err := s.Vendors.Create(c.Request.Context(), in)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	s.recordVendor(c, "vendor_create", "新增厂商 "+v.Name)
	resp.OK(c, v)
}

func (s *Server) updateVendor(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var in vendors.Input
	if !resp.BindJSON(c, &in) {
		return
	}
	v, err := s.Channels.UpdateVendor(c.Request.Context(), id, in)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	s.recordVendor(c, "vendor_update", fmt.Sprintf("修改厂商 %s（生成 %d 个分组改名草稿）", v.Name, v.GroupDrafts))
	resp.OK(c, v)
}

func (s *Server) deleteVendor(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	if err := s.Vendors.Delete(c.Request.Context(), id); err != nil {
		resp.FailFrom(c, err)
		return
	}
	s.recordVendor(c, "vendor_delete", "删除厂商")
	resp.OK(c, nil)
}

// PUT /vendors/order {ids:[...]}: position in the list = position in AutoGroups. NewAPI is
// written through POST /vendor-pricing/apply, where the order change shows up as an
// AutoGroups change.
func (s *Server) reorderVendors(c *gin.Context) {
	var in struct {
		IDs []uint `json:"ids"`
	}
	if !resp.BindJSON(c, &in) {
		return
	}
	if err := s.Vendors.Reorder(c.Request.Context(), in.IDs); err != nil {
		resp.FailFrom(c, err)
		return
	}
	s.recordVendor(c, "vendor_reorder", "调整厂商顺序")
	resp.OK(c, nil)
}
