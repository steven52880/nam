package api

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/cachetest"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/scheduler"
	"github.com/steven52880/nam/internal/store"
)

func (s *Server) listNewAPIChannels(c *gin.Context) {
	chs, err := s.Ports.Channels.List(c.Request.Context())
	if err != nil {
		resp.Fail(c, http.StatusBadGateway, err)
		return
	}
	for i := range chs {
		chs[i].Key = ""
	}
	resp.OK(c, gin.H{"total": len(chs), "items": chs})
}

func (s *Server) getSession(c *gin.Context) {
	id, _ := s.Ports.Auth.Identity(c)
	resp.OK(c, gin.H{"mode": s.Ports.Auth.Mode(), "identity": id, "version": s.Version})
}

func (s *Server) getBackend(c *gin.Context) {
	info, err := s.Ports.Backend.Info(c.Request.Context())
	if err != nil {
		info.Mode = s.Ports.Auth.Mode()
		info.LastError = err.Error()
	}
	if s.Stats != nil {
		info.PatchP1 = info.PatchP1 || s.Stats.PatchP1(c.Request.Context())
	}
	resp.OK(c, info)
}

func (s *Server) listJobs(c *gin.Context) {
	resp.OK(c, s.Sched.Statuses(c.Request.Context()))
}

// POST /jobs/:name?wait=1
func (s *Server) triggerJob(c *gin.Context) {
	wait := c.Query("wait") == "1" || c.Query("wait") == "true"
	ctx := c.Request.Context()
	if wait {
		var cancel func()
		ctx, cancel = contextTimeout(ctx, 10*time.Minute)
		defer cancel()
	}
	if err := s.Sched.Trigger(ctx, c.Param("name"), wait); err != nil {
		if errors.Is(err, scheduler.ErrUnknownJob) {
			resp.Fail(c, http.StatusNotFound, err)
			return
		}
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, nil)
}

// settingKeys are the nam settings exposed through /settings. Values are strings.
var settingKeys = []string{
	store.KeySiteRate, store.KeyProtectThreshold, store.KeyPricingAutoWrite, store.KeyPricingMinChange,
	store.KeyLogRetentionDays, store.KeyStatsRetentionDays, store.KeyNewAPIURL,
	store.KeyStatsBackfillDays, store.KeyNewAPILogCleanup, store.KeyUSDCNYRate,
}

// Write-only token setting and its read-only hint (the token itself is never returned).
const (
	settingTestToken     = "newapi_test_token"
	settingTestTokenHint = "newapi_test_token_hint"
)

func (s *Server) getSettings(c *gin.Context) {
	ctx := c.Request.Context()
	out := map[string]string{}
	for _, k := range settingKeys {
		if v, found, err := s.Store.GetKV(ctx, k); err == nil && found {
			out[k] = v
		}
	}
	if v, found, err := s.Store.GetKV(ctx, store.KeyCacheTestTokenHint); err == nil && found && v != "" {
		out[settingTestTokenHint] = v
	}
	for _, j := range s.Sched.Statuses(ctx) {
		out[store.KeyJobIntervalPrefix+j.Name] = (time.Duration(j.IntervalS) * time.Second).String()
	}
	resp.OK(c, out)
}

// putSettings updates nam-only settings. Site rate changes are rejected here: they
// write NewAPI and go through the pricing flow (M5) with secure verification.
func (s *Server) putSettings(c *gin.Context) {
	var in map[string]string
	if err := c.ShouldBindJSON(&in); err != nil {
		resp.Fail(c, http.StatusBadRequest, resp.Invalid("请求体必须是字符串键值对"))
		return
	}
	ctx := c.Request.Context()
	for k, v := range in {
		if err := validateSetting(k, v); err != nil {
			resp.FailFrom(c, err)
			return
		}
	}
	for k, v := range in {
		if k == settingTestToken {
			if err := s.saveTestToken(c, v); err != nil {
				resp.FailFrom(c, err)
				return
			}
			continue
		}
		if k == store.KeyNewAPIURL {
			v, _ = cachetest.NormalizeURL(v) // validated above
		}
		before, _, _ := s.Store.GetKV(ctx, k)
		if err := s.Store.SetKV(ctx, k, v); err != nil {
			resp.FailFrom(c, err)
			return
		}
		s.Audit.Record(ctx, store.AuditLog{Kind: store.AuditSetting, Target: "nam:" + k, Action: "set", Summary: "修改 nam 设置 " + k, Before: before, After: v})
	}
	s.getSettings(c)
}

// saveTestToken seals the NewAPI test token. The audit row carries only hints, never the token.
func (s *Server) saveTestToken(c *gin.Context, raw string) error {
	ctx := c.Request.Context()
	before, _, _ := s.Store.GetKV(ctx, store.KeyCacheTestTokenHint)
	if err := cachetest.SaveToken(ctx, s.Store, s.Box, raw); err != nil {
		return err
	}
	after, _, _ := s.Store.GetKV(ctx, store.KeyCacheTestTokenHint)
	s.Audit.Record(ctx, store.AuditLog{Kind: store.AuditSetting, Target: "nam:" + settingTestToken, Action: "set",
		Summary: "修改 NewAPI 测试令牌", Before: before, After: after})
	return nil
}

func validateSetting(k, v string) error {
	switch {
	case k == settingTestToken:
		if strings.TrimSpace(v) == "" {
			return nil // clears the token
		}
		_, err := cachetest.CleanToken(v)
		return err
	case k == store.KeyNewAPIURL:
		_, err := cachetest.NormalizeURL(v)
		return err
	case k == store.KeySiteRate:
		return resp.Invalid("站点汇率需通过定价流程修改（M5）")
	case k == store.KeyProtectThreshold:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f <= 0 || math.IsNaN(f) {
			return resp.Invalid("变化保护阈值必须大于 0")
		}
	case k == store.KeyUSDCNYRate:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || !(f > 0) || f > 100 {
			return resp.Invalid("美元参考汇率必须是大于 0 且不超过 100 的数字")
		}
	case k == store.KeyPricingAutoWrite:
		if v != "true" && v != "false" {
			return resp.Invalid("自动写入开关必须是 true 或 false")
		}
	case k == store.KeyPricingMinChange:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || math.IsNaN(f) || f < 0 || f >= 1 {
			return resp.Invalid("最小变动必须是 0（含）到 1（不含）之间的小数")
		}
	case k == store.KeyLogRetentionDays || k == store.KeyStatsRetentionDays:
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return resp.Invalid("保留天数必须是非负整数")
		}
	case k == store.KeyStatsBackfillDays:
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 90 {
			return resp.Invalid("首次回填天数必须是 1–90 的整数")
		}
	case k == store.KeyNewAPILogCleanup:
		if v != "true" && v != "false" {
			return resp.Invalid("清理 NewAPI 日志开关必须是 true 或 false")
		}
	case strings.HasPrefix(k, store.KeyJobIntervalPrefix):
		d, err := time.ParseDuration(v)
		if err != nil || d < time.Minute {
			return resp.Invalid("调度间隔至少 1 分钟，格式如 30m、6h")
		}
	default:
		return resp.Invalid("未知设置项 " + k)
	}
	return nil
}

func (s *Server) listAudit(c *gin.Context) {
	atoi := func(k string) int { n, _ := strconv.Atoi(c.Query(k)); return n }
	atoi64 := func(k string) int64 { n, _ := strconv.ParseInt(c.Query(k), 10, 64); return n }
	rows, total, err := s.Store.ListAudit(c.Request.Context(), store.AuditFilter{
		Kind: c.Query("kind"), Trigger: c.Query("trigger"), Result: c.Query("result"),
		From: atoi64("from"), To: atoi64("to"), Q: c.Query("q"),
		Limit: atoi("limit"), Offset: atoi("offset"),
	})
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"total": total, "items": rows})
}

func (s *Server) getAudit(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	a, err := s.Store.GetAudit(c.Request.Context(), id)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, a)
}
