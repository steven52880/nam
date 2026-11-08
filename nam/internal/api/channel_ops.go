package api

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/channels"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/ports"
)

func (s *Server) channelDrift(c *gin.Context) {
	id, okID := parseChannelID(c)
	if !okID {
		return
	}
	d, err := s.Channels.Drift(c.Request.Context(), id)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, d)
}

func (s *Server) acceptDrift(c *gin.Context) {
	id, okID := parseChannelID(c)
	if !okID {
		return
	}
	if err := s.Channels.AcceptDrift(c.Request.Context(), id); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, nil)
}

// POST /channels/:id/drift/overwrite creates an update draft; the user applies it via the normal flow.
func (s *Server) overwriteDrift(c *gin.Context) {
	id, okID := parseChannelID(c)
	if !okID {
		return
	}
	if err := s.Channels.OverwriteDrift(c.Request.Context(), id); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, nil)
}

func (s *Server) adoptChannel(c *gin.Context) {
	id, okID := parseChannelID(c)
	if !okID {
		return
	}
	var ext channels.ExtFields
	if !resp.BindJSON(c, &ext) {
		return
	}
	if err := s.Channels.Adopt(c.Request.Context(), id, ext); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, nil)
}

// POST /channels/:id/test {model}: NewAPI's own channel test, through the audited port.
func (s *Server) testChannel(c *gin.Context) {
	id, okID := parseChannelID(c)
	if !okID {
		return
	}
	var in struct {
		Model string `json:"model"`
	}
	if c.Request.ContentLength != 0 && !resp.BindJSON(c, &in) {
		return
	}
	ctx, cancel := withTimeout(c, 2*time.Minute)
	defer cancel()
	res, err := s.Ports.Channels.Test(ctx, id, in.Model)
	if err != nil {
		resp.FailFrom(c, resp.Upstream(err))
		return
	}
	resp.OK(c, res)
}

// GET /channels/:id/newapi-link → {url}. Embedded mode is same-origin, so the url is a path.
func (s *Server) channelLink(c *gin.Context) {
	id, okID := parseChannelID(c)
	if !okID {
		return
	}
	path := "/console/channel?id=" + strconv.Itoa(id)
	info, _ := s.Ports.Backend.Info(c.Request.Context())
	if s.Ports.Auth.Mode() == ports.ModeStandalone && info.Address != "" {
		path = strings.TrimRight(info.Address, "/") + path
	}
	resp.OK(c, gin.H{"url": path})
}
