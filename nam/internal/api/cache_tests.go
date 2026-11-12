package api

import (
	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/cachetest"
	"github.com/steven52880/nam/internal/resp"
)

// File is not named cache_test.go on purpose: that suffix makes Go treat it as a test file.
func (s *Server) registerCacheTestRoutes(p *gin.RouterGroup) {
	if s.CacheTest == nil {
		return
	}
	p.POST("/channels/:id/cache-test", s.startCacheTest)
	p.GET("/cache-tests/latest", s.latestCacheTests)
	p.GET("/cache-tests/:id", s.getCacheTest)
}

// POST /channels/:id/cache-test {model, rounds?, protocol?, mode?} → {test_id}
// mode "direct" (default) calls the upstream itself; "newapi" goes through the NewAPI relay.
// The test runs in the background; poll GET /cache-tests/:id. The audit row is written
// by the runner when the test finishes.
func (s *Server) startCacheTest(c *gin.Context) {
	id, okID := parseChannelID(c)
	if !okID {
		return
	}
	var in struct {
		Model    string `json:"model"`
		Rounds   int    `json:"rounds"`
		Protocol string `json:"protocol"`
		Mode     string `json:"mode"` // direct (default) | newapi
	}
	if !resp.BindJSON(c, &in) {
		return
	}
	testID, err := s.CacheTest.Start(c.Request.Context(), cachetest.StartInput{
		ChannelID: id, Model: in.Model, Rounds: in.Rounds, Protocol: in.Protocol, Mode: in.Mode,
	})
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"test_id": testID})
}

// GET /cache-tests/:id → the full record (status running|ok|fail).
func (s *Server) getCacheTest(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	t, err := s.CacheTest.Get(c.Request.Context(), id)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, t)
}

// GET /cache-tests/latest → {items: [latest record per channel]}
func (s *Server) latestCacheTests(c *gin.Context) {
	ts, err := s.CacheTest.Latest(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"items": ts})
}
