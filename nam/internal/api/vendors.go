package api

import (
	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/internal/vendors"
)

func (s *Server) registerVendorRoutes(p *gin.RouterGroup) {
	p.GET("/vendors", s.listVendors)
	p.POST("/vendors", s.createVendor)
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
	v, err := s.Vendors.Update(c.Request.Context(), id, in)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	s.recordVendor(c, "vendor_update", "修改厂商 "+v.Name)
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

// PUT /vendors/order {ids:[...]}: position in the list = position in AutoGroups (written in M5).
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
	// TODO(M5): write GroupRatio/AutoGroups/UserUsableGroups to NewAPI from the vendor list.
	resp.OK(c, nil)
}
