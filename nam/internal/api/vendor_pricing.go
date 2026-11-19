package api

import (
	"errors"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/internal/vendorprice"
)

// registerVendorPricingRoutes mounts /vendor-pricing. Static paths come before /:id.
func (s *Server) registerVendorPricingRoutes(p *gin.RouterGroup) {
	p.GET("/vendor-pricing", s.getVendorPricing)
	p.POST("/vendor-pricing/preview", s.previewVendorPricing)
	p.POST("/vendor-pricing/apply", s.applyVendorPricing)
	p.PUT("/vendor-pricing/:id", s.updateVendorPricing)
	p.POST("/vendor-pricing/:id/confirm", s.confirmVendorPricing)
}

func (s *Server) getVendorPricing(c *gin.Context) {
	out, err := s.VendorPrice.Overview(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, out)
}

func (s *Server) updateVendorPricing(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var in vendorprice.UpdateInput
	if !resp.BindJSON(c, &in) {
		return
	}
	it, err := s.VendorPrice.Update(c.Request.Context(), id, in)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, it)
}

// bindApplyInput reads the optional body of preview/apply (an empty body = all defaults).
func bindApplyInput(c *gin.Context) (vendorprice.ApplyInput, bool) {
	var in vendorprice.ApplyInput
	if err := c.ShouldBindJSON(&in); err != nil && !errors.Is(err, io.EOF) {
		resp.Fail(c, 400, resp.Invalid("请求体不是合法 JSON："+err.Error()))
		return in, false
	}
	return in, true
}

func (s *Server) previewVendorPricing(c *gin.Context) {
	in, okIn := bindApplyInput(c)
	if !okIn {
		return
	}
	out, err := s.VendorPrice.Preview(c.Request.Context(), in)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, out)
}

func (s *Server) applyVendorPricing(c *gin.Context) {
	in, okIn := bindApplyInput(c)
	if !okIn {
		return
	}
	out, err := s.VendorPrice.Apply(c.Request.Context(), in, store.TriggerManual)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, out)
}

func (s *Server) confirmVendorPricing(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	out, err := s.VendorPrice.Confirm(c.Request.Context(), id)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, out)
}
