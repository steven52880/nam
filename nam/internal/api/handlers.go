package api

import (
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/store"
)

// Credential is the plaintext credential payload (sealed into Upstream.Cred).
type Credential struct {
	AccessToken string `json:"access_token,omitempty"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	Key         string `json:"key,omitempty"`
}

func (c Credential) empty() bool {
	return c.AccessToken == "" && c.Username == "" && c.Password == "" && c.Key == ""
}

// UpstreamInput is the create/update body.
type UpstreamInput struct {
	Name           *string     `json:"name"`
	Type           *string     `json:"type"`
	BaseURL        *string     `json:"base_url"`
	CredType       *string     `json:"cred_type"`
	Credential     *Credential `json:"credential"`
	PlatformUserID *int        `json:"platform_user_id"`
	CurrencyRate   *float64    `json:"currency_rate"`
	Tags           *string     `json:"tags"`
	Remark         *string     `json:"remark"`
	AdapterConfig  *string     `json:"adapter_config"`
}

// UpstreamView is an upstream in API responses.
type UpstreamView struct {
	store.Upstream
	HasCredential bool     `json:"has_credential"`
	BalanceCNY    *float64 `json:"balance_cny"`
	TokenCount    int      `json:"token_count"`
	ChannelCount  int      `json:"channel_count"`
}

var upstreamTypes = map[string]bool{
	store.UpstreamNewAPI: true, store.UpstreamSKKey: true, store.UpstreamSub2API: true,
	store.UpstreamOneHub: true, store.UpstreamOfficial: true, store.UpstreamGeneric: true,
}

var credTypes = map[string]bool{"": true, store.CredAccessToken: true, store.CredPassword: true, store.CredSKKey: true}

func view(u store.Upstream, tokens, channels map[uint]int) UpstreamView {
	v := UpstreamView{Upstream: u, HasCredential: len(u.Cred) > 0, TokenCount: tokens[u.ID], ChannelCount: channels[u.ID]}
	if u.BalanceUSD != nil {
		cny := *u.BalanceUSD * u.CurrencyRate
		v.BalanceCNY = &cny
	}
	return v
}

func (s *Server) listUpstreams(c *gin.Context) {
	ctx := c.Request.Context()
	us, err := s.Store.ListUpstreams(ctx)
	if err != nil {
		failFrom(c, err)
		return
	}
	tk, ch, err := s.Store.UpstreamCounts(ctx)
	if err != nil {
		failFrom(c, err)
		return
	}
	out := make([]UpstreamView, 0, len(us))
	for _, u := range us {
		out = append(out, view(u, tk, ch))
	}
	ok(c, out)
}

func parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		fail(c, http.StatusBadRequest, invalid("无效的 id"))
		return 0, false
	}
	return uint(id), true
}

func (s *Server) getUpstream(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	u, err := s.Store.GetUpstream(c.Request.Context(), id)
	if err != nil {
		failFrom(c, err)
		return
	}
	tk, ch, _ := s.Store.UpstreamCounts(c.Request.Context())
	ok(c, view(u, tk, ch))
}

func (s *Server) applyUpstreamInput(u *store.Upstream, in UpstreamInput) error {
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if n == "" || len(n) > 64 {
			return invalid("名称不能为空，且不超过 64 个字符")
		}
		u.Name = n
	}
	if in.Type != nil {
		if !upstreamTypes[*in.Type] {
			return invalid("未知的上游类型")
		}
		u.Type = *in.Type
	}
	if in.BaseURL != nil {
		raw := strings.TrimRight(strings.TrimSpace(*in.BaseURL), "/")
		pu, err := url.Parse(raw)
		if err != nil || (pu.Scheme != "http" && pu.Scheme != "https") || pu.Host == "" {
			return invalid("站点地址必须是 http(s):// 开头的根地址")
		}
		u.BaseURL = raw
	}
	if in.CredType != nil {
		if !credTypes[*in.CredType] {
			return invalid("未知的凭证类型")
		}
		u.CredType = *in.CredType
	}
	if in.Credential != nil {
		if in.Credential.empty() {
			u.Cred = nil
		} else {
			b, _ := json.Marshal(in.Credential)
			sealed, err := s.Box.Seal(b)
			if err != nil {
				return err
			}
			u.Cred = sealed
		}
	}
	if in.PlatformUserID != nil {
		u.PlatformUserID = *in.PlatformUserID
	}
	if in.CurrencyRate != nil {
		r := *in.CurrencyRate
		if math.IsNaN(r) || math.IsInf(r, 0) || r <= 0 {
			return invalid("上游货币汇率必须大于 0")
		}
		u.CurrencyRate = r
	}
	if in.Tags != nil {
		u.Tags = strings.TrimSpace(*in.Tags)
	}
	if in.Remark != nil {
		u.Remark = *in.Remark
	}
	if in.AdapterConfig != nil {
		if *in.AdapterConfig != "" && !json.Valid([]byte(*in.AdapterConfig)) {
			return invalid("适配器配置必须是 JSON")
		}
		u.AdapterConfig = *in.AdapterConfig
	}
	return nil
}

func (s *Server) createUpstream(c *gin.Context) {
	var in UpstreamInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, invalid("请求体不是合法 JSON"))
		return
	}
	if in.Name == nil || in.Type == nil || in.BaseURL == nil {
		fail(c, http.StatusBadRequest, invalid("名称、类型和站点地址必填"))
		return
	}
	u := store.Upstream{CurrencyRate: 1}
	if err := s.applyUpstreamInput(&u, in); err != nil {
		failFrom(c, err)
		return
	}
	ctx := c.Request.Context()
	if err := s.Store.SaveUpstream(ctx, &u); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			fail(c, http.StatusConflict, invalid("上游名称已存在"))
			return
		}
		failFrom(c, err)
		return
	}
	s.Audit.Record(ctx, store.AuditLog{Kind: store.AuditUpstream, Target: "upstream:" + strconv.Itoa(int(u.ID)), Action: "create", Summary: "新增上游 " + u.Name})
	ok(c, view(u, nil, nil))
}

func (s *Server) updateUpstream(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var in UpstreamInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, invalid("请求体不是合法 JSON"))
		return
	}
	ctx := c.Request.Context()
	u, err := s.Store.GetUpstream(ctx, id)
	if err != nil {
		failFrom(c, err)
		return
	}
	if err := s.applyUpstreamInput(&u, in); err != nil {
		failFrom(c, err)
		return
	}
	if err := s.Store.SaveUpstream(ctx, &u); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			fail(c, http.StatusConflict, invalid("上游名称已存在"))
			return
		}
		failFrom(c, err)
		return
	}
	s.Audit.Record(ctx, store.AuditLog{Kind: store.AuditUpstream, Target: "upstream:" + strconv.Itoa(int(u.ID)), Action: "update", Summary: "修改上游 " + u.Name})
	tk, ch, _ := s.Store.UpstreamCounts(ctx)
	ok(c, view(u, tk, ch))
}

func (s *Server) deleteUpstream(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	ctx := c.Request.Context()
	u, err := s.Store.GetUpstream(ctx, id)
	if err != nil {
		failFrom(c, err)
		return
	}
	if err := s.Store.DeleteUpstream(ctx, id); err != nil {
		failFrom(c, err)
		return
	}
	// TODO(M2): add delete drafts for this upstream's channels.
	s.Audit.Record(ctx, store.AuditLog{Kind: store.AuditUpstream, Target: "upstream:" + strconv.Itoa(int(id)), Action: "delete", Summary: "删除上游 " + u.Name})
	ok(c, nil)
}

func (s *Server) revealCredential(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	ctx := c.Request.Context()
	u, err := s.Store.GetUpstream(ctx, id)
	if err != nil {
		failFrom(c, err)
		return
	}
	pt, err := s.Box.Open(u.Cred)
	if err != nil {
		failFrom(c, err)
		return
	}
	var cred Credential
	if len(pt) > 0 {
		_ = json.Unmarshal(pt, &cred)
	}
	s.Audit.Record(ctx, store.AuditLog{Kind: store.AuditUpstream, Target: "upstream:" + strconv.Itoa(int(id)), Action: "reveal", Summary: "查看上游凭证 " + u.Name})
	ok(c, cred)
}

func (s *Server) listNewAPIChannels(c *gin.Context) {
	chs, err := s.Ports.Channels.List(c.Request.Context())
	if err != nil {
		fail(c, http.StatusBadGateway, err)
		return
	}
	for i := range chs {
		chs[i].Key = ""
	}
	ok(c, gin.H{"total": len(chs), "items": chs})
}

func (s *Server) getSession(c *gin.Context) {
	id, _ := s.Ports.Auth.Identity(c)
	ok(c, gin.H{"mode": s.Ports.Auth.Mode(), "identity": id, "version": s.Version})
}

func (s *Server) getBackend(c *gin.Context) {
	info, err := s.Ports.Backend.Info(c.Request.Context())
	if err != nil {
		info.Mode = s.Ports.Auth.Mode()
		info.LastError = err.Error()
	}
	ok(c, info)
}

func (s *Server) listJobs(c *gin.Context) {
	ok(c, s.Sched.Statuses(c.Request.Context()))
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
		failFrom(c, err)
		return
	}
	ok(c, nil)
}

// settingKeys are the nam settings exposed through /settings. Values are strings.
var settingKeys = []string{
	store.KeySiteRate, store.KeyProtectThreshold, store.KeyAutoWrite,
	store.KeyLogRetentionDays, store.KeyStatsRetentionDays,
}

func (s *Server) getSettings(c *gin.Context) {
	ctx := c.Request.Context()
	out := map[string]string{}
	for _, k := range settingKeys {
		if v, found, err := s.Store.GetKV(ctx, k); err == nil && found {
			out[k] = v
		}
	}
	for _, j := range s.Sched.Statuses(ctx) {
		out[store.KeyJobIntervalPrefix+j.Name] = (time.Duration(j.IntervalS) * time.Second).String()
	}
	ok(c, out)
}

// putSettings updates nam-only settings. Site rate changes are rejected here: they
// write NewAPI and go through the pricing flow (M5) with secure verification.
func (s *Server) putSettings(c *gin.Context) {
	var in map[string]string
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, invalid("请求体必须是字符串键值对"))
		return
	}
	ctx := c.Request.Context()
	for k, v := range in {
		if err := validateSetting(k, v); err != nil {
			failFrom(c, err)
			return
		}
	}
	for k, v := range in {
		before, _, _ := s.Store.GetKV(ctx, k)
		if err := s.Store.SetKV(ctx, k, v); err != nil {
			failFrom(c, err)
			return
		}
		s.Audit.Record(ctx, store.AuditLog{Kind: store.AuditSetting, Target: "nam:" + k, Action: "set", Summary: "修改 nam 设置 " + k, Before: before, After: v})
	}
	s.getSettings(c)
}

func validateSetting(k, v string) error {
	switch {
	case k == store.KeySiteRate:
		return invalid("站点汇率需通过定价流程修改（M5）")
	case k == store.KeyProtectThreshold:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f <= 0 || math.IsNaN(f) {
			return invalid("变化保护阈值必须大于 0")
		}
	case k == store.KeyAutoWrite:
		if _, err := strconv.ParseBool(v); err != nil {
			return invalid("自动写入开关必须是 true/false")
		}
	case k == store.KeyLogRetentionDays || k == store.KeyStatsRetentionDays:
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return invalid("保留天数必须是非负整数")
		}
	case strings.HasPrefix(k, store.KeyJobIntervalPrefix):
		d, err := time.ParseDuration(v)
		if err != nil || d < time.Minute {
			return invalid("调度间隔至少 1 分钟，格式如 30m、6h")
		}
	default:
		return invalid("未知设置项 " + k)
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
		failFrom(c, err)
		return
	}
	ok(c, gin.H{"total": total, "items": rows})
}

func (s *Server) getAudit(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	a, err := s.Store.GetAudit(c.Request.Context(), id)
	if err != nil {
		failFrom(c, err)
		return
	}
	ok(c, a)
}
