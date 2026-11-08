// Package api implements /api/nam/* HTTP handlers. Contract: docs/api/openapi.yaml.
package api

import (
	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/audit"
	"github.com/steven52880/nam/internal/channels"
	"github.com/steven52880/nam/internal/crypto"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/scheduler"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/internal/upstream"
	"github.com/steven52880/nam/internal/vendors"
	"github.com/steven52880/nam/ports"
)

// Deps are the core services handlers use.
type Deps struct {
	Store    *store.Store
	Box      *crypto.Box
	Audit    *audit.Recorder
	Sched    *scheduler.Scheduler
	Ports    ports.Ports // Channels/Options are already audit-wrapped
	Channels *channels.Service
	Syncer   *upstream.Syncer
	Vendors  *vendors.Service
	Version  string
}

// Server holds handler dependencies.
type Server struct {
	Deps
}

// Register mounts all routes on g (= /api/nam). Public routes are mounted before auth.
func (s *Server) Register(g *gin.RouterGroup) {
	auth := s.Ports.Auth
	g.GET("/ping", func(c *gin.Context) { resp.OK(c, "pong") })
	if rr, okRR := auth.(ports.RouteRegistrar); okRR {
		rr.RegisterPublicRoutes(g)
	}

	p := g.Group("", auth.Middleware(), s.operatorCtx())
	if rr, okRR := auth.(ports.RouteRegistrar); okRR {
		rr.RegisterPrivateRoutes(p)
	}
	p.GET("/session", s.getSession)
	p.GET("/backend", s.getBackend)

	// upstreams (nam.db only)
	p.GET("/upstreams", s.listUpstreams)
	p.POST("/upstreams", s.createUpstream)
	p.GET("/upstreams/:id", s.getUpstream)
	p.PUT("/upstreams/:id", s.updateUpstream)
	p.DELETE("/upstreams/:id", s.deleteUpstream)
	p.GET("/upstreams/:id/credential", auth.SecureMiddleware(), s.revealCredential)
	s.registerUpstreamRoutes(p)
	s.registerVendorRoutes(p)
	s.registerChannelRoutes(p)

	// raw NewAPI channels (read only passthrough, for M1 acceptance and debugging)
	p.GET("/newapi/channels", s.listNewAPIChannels)

	// jobs
	p.GET("/jobs", s.listJobs)
	p.POST("/jobs/:name", s.triggerJob)

	// settings
	p.GET("/settings", s.getSettings)
	p.PUT("/settings", s.putSettings)

	// audit
	p.GET("/audit", s.listAudit)
	p.GET("/audit/:id", s.getAudit)
}

const ctxIdentity = "nam.identity"

// operatorCtx copies the identity into the request context for audit attribution.
func (s *Server) operatorCtx() gin.HandlerFunc {
	return func(c *gin.Context) {
		if id, okID := s.Ports.Auth.Identity(c); okID {
			c.Set(ctxIdentity, id)
			ctx := audit.WithOperator(c.Request.Context(), id.Username)
			ctx = audit.WithTrigger(ctx, store.TriggerManual)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	}
}
