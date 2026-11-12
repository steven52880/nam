package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/internal/upstream"
)

func (s *Server) registerUpstreamRoutes(p *gin.RouterGroup) {
	p.POST("/upstreams/probe", s.probeUpstream)
	p.POST("/upstreams/test", s.testDraftUpstream)
	p.POST("/upstreams/:id/test", s.testUpstream)
	p.POST("/upstreams/:id/sync", s.syncUpstream)
	p.GET("/upstreams/:id/tokens", s.listTokens)
	p.POST("/upstreams/:id/tokens", s.addManualToken)
	p.POST("/upstreams/:id/tokens/remote", s.createRemoteToken)
	p.DELETE("/upstreams/:id/tokens/:tid", s.deleteToken)
	p.GET("/upstreams/:id/groups", s.listGroups)
	p.GET("/upstreams/:id/models", s.listUpstreamModels)
	p.POST("/upstreams/:id/models/fetch", s.fetchUpstreamModels)
	p.POST("/upstreams/:id/derive", s.deriveUpstream)
}

func withTimeout(c *gin.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), d)
}

// ProbeResponse reports what auto-detection found.
type ProbeResponse struct {
	upstream.ProbeResult
	Adapter string `json:"adapter"` // which adapter recognised the site
}

// POST /upstreams/probe {base_url}: tries the newapi adapter, then generic.
func (s *Server) probeUpstream(c *gin.Context) {
	var in struct {
		BaseURL string `json:"base_url"`
	}
	if !resp.BindJSON(c, &in) {
		return
	}
	base, err := normalizeBaseURL(in.BaseURL)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	ctx, cancel := withTimeout(c, 30*time.Second)
	defer cancel()
	r, err := s.Syncer.ProbeSite(ctx, base)
	if err != nil {
		resp.FailFrom(c, resp.Upstream(err))
		return
	}
	resp.OK(c, ProbeResponse{ProbeResult: r, Adapter: r.Type})
}

// siteFromInput builds a Site from an unsaved upstream form (wizard test).
func siteFromInput(in UpstreamInput) (upstream.Site, error) {
	if in.Type == nil || in.BaseURL == nil {
		return upstream.Site{}, resp.Invalid("类型和站点地址必填")
	}
	if !upstreamTypes[*in.Type] {
		return upstream.Site{}, resp.Invalid("未知的上游类型")
	}
	base, err := normalizeBaseURL(*in.BaseURL)
	if err != nil {
		return upstream.Site{}, err
	}
	site := upstream.Site{Type: *in.Type, BaseURL: base}
	if in.CredType != nil {
		if !credTypes[*in.CredType] {
			return site, resp.Invalid("未知的凭证类型")
		}
		site.CredType = *in.CredType
	}
	if in.PlatformUserID != nil {
		site.UserID = *in.PlatformUserID
	}
	var cred *Credential
	if in.Credential != nil {
		cp := *in.Credential
		cred = &cp
	}
	if in.CredType != nil || in.Credential != nil {
		if err := checkSessionCred(site.Type, site.CredType, site.UserID, cred); err != nil {
			return site, err
		}
	}
	if cred != nil {
		site.Cred = upstream.Credential(*cred)
	}
	if in.AdapterConfig != nil {
		site.Config = *in.AdapterConfig
	}
	if site.Type == store.UpstreamGeneric {
		if err := upstream.ValidateGenericConfig(site.Config); err != nil {
			return site, resp.Invalid(err.Error())
		}
	}
	return site, nil
}

// POST /upstreams/test: run the wizard checks without saving anything.
func (s *Server) testDraftUpstream(c *gin.Context) {
	var in UpstreamInput
	if !resp.BindJSON(c, &in) {
		return
	}
	site, err := siteFromInput(in)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	ctx, cancel := withTimeout(c, 2*time.Minute)
	defer cancel()
	resp.OK(c, s.Syncer.Test(ctx, site))
}

// POST /upstreams/:id/test: run the wizard checks against a saved upstream.
func (s *Server) testUpstream(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	u, err := s.Store.GetUpstream(c.Request.Context(), id)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	site, err := upstream.SiteFor(u, s.Box)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	site.Creds = upstream.NewCredStore(s.Store, s.Box, u.ID)
	ctx, cancel := withTimeout(c, 2*time.Minute)
	defer cancel()
	resp.OK(c, s.Syncer.Test(ctx, site))
}

// SyncResult is the outcome of one synced part.
type SyncResult struct {
	What  string `json:"what"`
	OK    bool   `json:"ok"`
	Count int    `json:"count"`
	Error string `json:"error,omitempty"`
}

// POST /upstreams/:id/sync {what:["tokens","groups","balance"]} (default: all three).
func (s *Server) syncUpstream(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var in struct {
		What []string `json:"what"`
	}
	if c.Request.ContentLength != 0 && !resp.BindJSON(c, &in) {
		return
	}
	if len(in.What) == 0 {
		in.What = []string{upstream.WhatTokens, upstream.WhatGroups, upstream.WhatBalance}
	}
	for _, w := range in.What {
		if w != upstream.WhatTokens && w != upstream.WhatGroups && w != upstream.WhatBalance {
			resp.FailFrom(c, resp.Invalid("未知的同步项 "+w))
			return
		}
	}
	if _, err := s.Store.GetUpstream(c.Request.Context(), id); err != nil {
		resp.FailFrom(c, err)
		return
	}
	ctx, cancel := withTimeout(c, 5*time.Minute)
	defer cancel()
	out := make([]SyncResult, 0, len(in.What))
	for _, w := range in.What {
		n, err := s.Syncer.Sync(ctx, id, w)
		r := SyncResult{What: w, OK: err == nil, Count: n}
		if err != nil {
			r.Error = err.Error()
		}
		out = append(out, r)
	}
	resp.OK(c, out)
}

func (s *Server) listGroups(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var gs []store.UpstreamGroup
	if err := s.Store.DB.WithContext(c.Request.Context()).Where("upstream_id = ?", id).Order("name").Find(&gs).Error; err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gs)
}

func (s *Server) listUpstreamModels(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var ms []store.UpstreamModelPrice
	if err := s.Store.DB.WithContext(c.Request.Context()).Where("upstream_id = ?", id).Order("model, `group`").Find(&ms).Error; err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, ms)
}

// POST /upstreams/:id/models/fetch {token_id}: list models with that token's key.
func (s *Server) fetchUpstreamModels(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var in struct {
		TokenID     uint `json:"token_id"`
		ChannelType int  `json:"channel_type"`
	}
	if !resp.BindJSON(c, &in) {
		return
	}
	if in.ChannelType == 0 {
		in.ChannelType = 58
	}
	ctx, cancel := withTimeout(c, time.Minute)
	defer cancel()
	var tok store.UpstreamToken
	if err := s.Store.DB.WithContext(ctx).Where("id = ? AND upstream_id = ?", in.TokenID, id).First(&tok).Error; err != nil {
		resp.Fail(c, http.StatusNotFound, resp.Invalid("令牌不存在"))
		return
	}
	key, err := s.Box.Open(tok.Key)
	if err != nil || len(key) == 0 {
		resp.FailFrom(c, resp.Invalid("该令牌没有保存 key"))
		return
	}
	u, err := s.Store.GetUpstream(ctx, id)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	site, err := upstream.SiteFor(u, s.Box)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	site.HTTP = s.Syncer.HTTP
	site.Creds = upstream.NewCredStore(s.Store, s.Box, u.ID)
	var fetchErr error
	if s.Ports.Models != nil {
		models, err := s.Ports.Models.FetchModels(ctx, site.BaseURL, in.ChannelType, string(key))
		if err == nil {
			resp.OK(c, gin.H{"models": dedupeSorted(models)})
			return
		}
		fetchErr = err
	}
	ad, err := upstream.Get(u.Type)
	if err != nil {
		resp.FailFrom(c, resp.Invalid(err.Error()))
		return
	}
	models, err := ad.ListModels(ctx, site, string(key))
	if err != nil {
		if fetchErr != nil {
			err = fetchErr
		}
		resp.FailFrom(c, resp.Upstream(err))
		return
	}
	resp.OK(c, gin.H{"models": dedupeSorted(models)})
}
