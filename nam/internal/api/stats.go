package api

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/stats"
)

func (s *Server) registerStatsRoutes(p *gin.RouterGroup) {
	p.GET("/stats/channels", s.statsChannels)
	p.GET("/stats/channels/:id/models", s.statsChannelModels)
	p.GET("/stats/overview", s.statsOverview)
	p.GET("/stats/series", s.statsSeries)
	p.GET("/stats/status", s.statsStatus)
	p.POST("/stats/rebuild", s.statsRebuild)
}

// maxModelParam caps the length (in characters) of a model query parameter.
const maxModelParam = 200

// windowParam defaults to "latest"; the raw value (e.g. "12h", "30d") is passed through and
// validity is checked by the stats package.
func windowParam(c *gin.Context) string {
	if w := c.Query("window"); w != "" {
		return w
	}
	return stats.WindowLatest
}

// int64Query parses a required integer query parameter or writes a 400.
func int64Query(c *gin.Context, name string) (int64, bool) {
	v, err := strconv.ParseInt(c.Query(name), 10, 64)
	if err != nil {
		resp.FailFrom(c, resp.Invalid("参数 "+name+" 必须是整数"))
		return 0, false
	}
	return v, true
}

func (s *Server) statsChannels(c *gin.Context) {
	model := strings.TrimSpace(c.Query("model"))
	if utf8.RuneCountInString(model) > maxModelParam {
		resp.FailFrom(c, resp.Invalid("model 不能超过 200 个字符"))
		return
	}
	out, err := s.Stats.ChannelStats(c.Request.Context(), windowParam(c), model)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, out)
}

func (s *Server) statsChannelModels(c *gin.Context) {
	id, ok := parseChannelID(c)
	if !ok {
		return
	}
	out, err := s.Stats.ChannelModels(c.Request.Context(), id, windowParam(c))
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, out)
}

func (s *Server) statsOverview(c *gin.Context) {
	from, ok := int64Query(c, "from")
	if !ok {
		return
	}
	to, ok := int64Query(c, "to")
	if !ok {
		return
	}
	out, err := s.Stats.Overview(c.Request.Context(), from, to, c.Query("dim"))
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, out)
}

func (s *Server) statsSeries(c *gin.Context) {
	from, ok := int64Query(c, "from")
	if !ok {
		return
	}
	to, ok := int64Query(c, "to")
	if !ok {
		return
	}
	out, err := s.Stats.Series(c.Request.Context(), from, to, c.Query("dim"), c.Query("key"), c.Query("bucket"))
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, out)
}

func (s *Server) statsStatus(c *gin.Context) {
	out, err := s.Stats.Status(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, out)
}

// POST /stats/rebuild {days}
func (s *Server) statsRebuild(c *gin.Context) {
	var in struct {
		Days int `json:"days"`
	}
	if !resp.BindJSON(c, &in) {
		return
	}
	if err := s.Stats.Rebuild(c.Request.Context(), in.Days); err != nil {
		if errors.Is(err, stats.ErrBadDays) {
			err = resp.Invalid(err.Error())
		}
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, nil)
}
