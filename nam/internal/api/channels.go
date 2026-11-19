package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/channels"
	"github.com/steven52880/nam/internal/resp"
)

func (s *Server) registerChannelRoutes(p *gin.RouterGroup) {
	p.GET("/channels", s.listChannels)
	p.PATCH("/channels/drafts", s.saveDrafts)
	p.DELETE("/channels/drafts", s.discardDrafts)
	p.POST("/channels/batch/preview", s.batchPreview)
	p.POST("/channels/batch", s.batchCreate)
	p.POST("/channels/match-tokens", s.matchTokens)
	p.POST("/channels/delete", s.markDelete)
	p.GET("/channels/plan", s.channelPlan)
	p.GET("/channels/costs", s.channelCosts)
	p.GET("/channels/issues", s.channelIssues)
	p.POST("/channels/apply", s.applyChannels)
	p.POST("/channels/:id/adopt", s.adoptChannel)
	p.POST("/channels/:id/test", s.testChannel)
	p.GET("/channels/:id/newapi-link", s.channelLink)
}

func issuesFail(c *gin.Context, issues []channels.ValidationIssue) {
	resp.FailData(c, http.StatusBadRequest, "校验未通过："+issues[0].Message, gin.H{"issues": issues})
}

func parseChannelID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		resp.FailFrom(c, resp.Invalid("无效的渠道 id"))
		return 0, false
	}
	return id, true
}

// GET /channels/issues: rule problems of every channel (drafts overlaid).
func (s *Server) channelIssues(c *gin.Context) {
	items, err := s.Channels.Issues(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"items": items})
}

// GET /channels/costs?model= (required): every channel's cost on one model.
func (s *Server) channelCosts(c *gin.Context) {
	model := strings.TrimSpace(c.Query("model"))
	if model == "" {
		resp.FailFrom(c, resp.Invalid("model 不能为空"))
		return
	}
	if utf8.RuneCountInString(model) > maxModelParam {
		resp.FailFrom(c, resp.Invalid("model 不能超过 200 个字符"))
		return
	}
	items, err := s.Channels.ModelCosts(c.Request.Context(), model)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"model": model, "items": items})
}

func (s *Server) listChannels(c *gin.Context) {
	ctx := c.Request.Context()
	rows, err := s.Channels.List(ctx)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	n, err := s.Channels.DraftCount(ctx)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"items": rows, "draft_count": n})
}

// PATCH /channels/drafts {changes:[Change]}
func (s *Server) saveDrafts(c *gin.Context) {
	var in struct {
		Changes []channels.Change `json:"changes"`
	}
	if !resp.BindJSON(c, &in) {
		return
	}
	if len(in.Changes) == 0 {
		resp.FailFrom(c, resp.Invalid("changes 不能为空"))
		return
	}
	issues, err := s.Channels.SaveChanges(c.Request.Context(), in.Changes)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	if len(issues) > 0 {
		issuesFail(c, issues)
		return
	}
	resp.OK(c, gin.H{"issues": []channels.ValidationIssue{}})
}

// DELETE /channels/drafts {ids?}: omitted ids = discard every draft.
func (s *Server) discardDrafts(c *gin.Context) {
	var in struct {
		IDs []int `json:"ids"`
	}
	if c.Request.ContentLength != 0 && !resp.BindJSON(c, &in) {
		return
	}
	if err := s.Channels.Discard(c.Request.Context(), in.IDs); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, nil)
}

func (s *Server) batchPreview(c *gin.Context) {
	var req channels.BatchRequest
	if !resp.BindJSON(c, &req) {
		return
	}
	res, err := s.Channels.BatchPreview(c.Request.Context(), req)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, res)
}

// POST /channels/batch: either a BatchRequest (expanded server-side) or {specs:[CreateSpec]}.
func (s *Server) batchCreate(c *gin.Context) {
	var in struct {
		channels.BatchRequest
		Specs []channels.CreateSpec `json:"specs"`
	}
	if !resp.BindJSON(c, &in) {
		return
	}
	ctx := c.Request.Context()
	specs := in.Specs
	if len(specs) == 0 {
		prev, err := s.Channels.BatchPreview(ctx, in.BatchRequest)
		if err != nil {
			resp.FailFrom(c, err)
			return
		}
		for _, it := range prev.Items {
			specs = append(specs, it.CreateSpec)
		}
	}
	if len(specs) == 0 {
		resp.FailFrom(c, resp.Invalid("没有可创建的渠道：检查令牌与模型是否已归属厂商"))
		return
	}
	ids, issues, err := s.Channels.AddCreateDrafts(ctx, specs)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	if len(issues) > 0 {
		issuesFail(c, issues)
		return
	}
	resp.OK(c, gin.H{"ids": ids})
}

func (s *Server) markDelete(c *gin.Context) {
	var in struct {
		IDs []int `json:"ids"`
	}
	if !resp.BindJSON(c, &in) {
		return
	}
	if len(in.IDs) == 0 {
		resp.FailFrom(c, resp.Invalid("ids 不能为空"))
		return
	}
	if err := s.Channels.MarkDelete(c.Request.Context(), in.IDs); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, nil)
}

// GET /channels/plan?ids=1,-2 : omitted = every draft.
func (s *Server) channelPlan(c *gin.Context) {
	var ids []int
	if q := strings.TrimSpace(c.Query("ids")); q != "" {
		for _, part := range strings.Split(q, ",") {
			n, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil {
				resp.FailFrom(c, resp.Invalid("ids 必须是逗号分隔的整数"))
				return
			}
			ids = append(ids, n)
		}
	}
	plan, err := s.Channels.Plan(c.Request.Context(), ids)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	if plan == nil {
		plan = []channels.PlanItem{}
	}
	resp.OK(c, plan)
}

// POST /channels/apply {ids?}
func (s *Server) applyChannels(c *gin.Context) {
	var in struct {
		IDs []int `json:"ids"`
	}
	if c.Request.ContentLength != 0 && !resp.BindJSON(c, &in) {
		return
	}
	ctx, cancel := withTimeout(c, 10*time.Minute)
	defer cancel()
	results, issues, err := s.Channels.Apply(ctx, in.IDs)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	if len(issues) > 0 {
		issuesFail(c, issues)
		return
	}
	success, failed := 0, 0
	for _, r := range results {
		if r.OK {
			success++
		} else {
			failed++
		}
	}
	if results == nil {
		results = []channels.Result{}
	}
	resp.OK(c, gin.H{"results": results, "success": success, "failed": failed})
}
