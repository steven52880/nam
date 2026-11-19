package api

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/channels"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

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

// POST /channels/match-tokens {upstream_id, channel_ids}: map channels to the upstream's
// tokens by key, recording a manual token for unmatched ones. Writes nam.db only.
func (s *Server) matchTokens(c *gin.Context) {
	var in struct {
		UpstreamID uint  `json:"upstream_id"`
		ChannelIDs []int `json:"channel_ids"`
	}
	if !resp.BindJSON(c, &in) {
		return
	}
	if in.UpstreamID == 0 || len(in.ChannelIDs) == 0 {
		resp.FailFrom(c, resp.Invalid("upstream_id 和 channel_ids 必填"))
		return
	}
	ctx := c.Request.Context()
	items, err := s.Channels.MatchTokens(ctx, in.UpstreamID, in.ChannelIDs)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	for _, it := range items {
		if it.Created {
			s.Audit.Record(ctx, store.AuditLog{Kind: store.AuditUpstream, Target: "upstream:" + strconv.Itoa(int(in.UpstreamID)),
				Action: "add_token", Summary: fmt.Sprintf("接管渠道 %d：未匹配到令牌，自动录入手动令牌 %s", it.ChannelID, it.TokenName)})
		}
	}
	if items == nil {
		items = []channels.MatchItem{}
	}
	resp.OK(c, gin.H{"items": items})
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
