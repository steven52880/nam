package api

import (
	"encoding/json"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/internal/upstream"
)

// Credential is the plaintext credential payload (sealed into Upstream.Cred).
type Credential struct {
	AccessToken  string `json:"access_token,omitempty"`
	Username     string `json:"username,omitempty"`
	Password     string `json:"password,omitempty"`
	Key          string `json:"key,omitempty"`
	Cookie       string `json:"cookie,omitempty"`        // value of the session cookie only, without "session="
	RefreshToken string `json:"refresh_token,omitempty"` // sub2api_session
	ExpiresAt    int64  `json:"expires_at,omitempty"`    // sub2api_session: access token expiry, Unix seconds
}

func (c Credential) empty() bool {
	return c.AccessToken == "" && c.Username == "" && c.Password == "" && c.Key == "" &&
		c.Cookie == "" && c.RefreshToken == "" && c.ExpiresAt == 0
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
	// CredExpiresAt is the estimated expiry of a cookie credential (Unix seconds), 0 if unknown.
	CredExpiresAt int64 `json:"cred_expires_at"`
}

var upstreamTypes = map[string]bool{
	store.UpstreamNewAPI: true, store.UpstreamSKKey: true, store.UpstreamSub2API: true,
	store.UpstreamOneHub: true, store.UpstreamOfficial: true, store.UpstreamGeneric: true,
}

var credTypes = map[string]bool{
	"": true, store.CredAccessToken: true, store.CredPassword: true, store.CredSKKey: true,
	store.CredCookie: true, store.CredSub2APISession: true,
}

func (s *Server) view(u store.Upstream, tokens, channels map[uint]int) UpstreamView {
	v := UpstreamView{Upstream: u, HasCredential: len(u.Cred) > 0, TokenCount: tokens[u.ID], ChannelCount: channels[u.ID]}
	if u.BalanceUSD != nil {
		cny := *u.BalanceUSD * u.CurrencyRate
		v.BalanceCNY = &cny
	}
	v.CredExpiresAt = s.cookieExpiry(u)
	return v
}

// cookieExpiry estimates when a cookie credential expires. Any decryption problem
// yields 0 so that listing upstreams never fails because of it.
func (s *Server) cookieExpiry(u store.Upstream) int64 {
	if u.CredType != store.CredCookie || len(u.Cred) == 0 || s.Box == nil {
		return 0
	}
	pt, err := s.Box.Open(u.Cred)
	if err != nil {
		return 0
	}
	var c upstream.Credential
	if json.Unmarshal(pt, &c) != nil {
		return 0
	}
	return upstream.CookieExpiresAt(c, u.CredUpdatedAt)
}

func (s *Server) listUpstreams(c *gin.Context) {
	ctx := c.Request.Context()
	us, err := s.Store.ListUpstreams(ctx)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	tk, ch, err := s.Store.UpstreamCounts(ctx)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	out := make([]UpstreamView, 0, len(us))
	for _, u := range us {
		out = append(out, s.view(u, tk, ch))
	}
	resp.OK(c, out)
}

func parseUintParam(c *gin.Context, name string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param(name), 10, 32)
	if err != nil || id == 0 {
		resp.FailFrom(c, resp.Invalid("无效的 "+name))
		return 0, false
	}
	return uint(id), true
}

func parseID(c *gin.Context) (uint, bool) { return parseUintParam(c, "id") }

func (s *Server) getUpstream(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	u, err := s.Store.GetUpstream(c.Request.Context(), id)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	tk, ch, _ := s.Store.UpstreamCounts(c.Request.Context())
	resp.OK(c, s.view(u, tk, ch))
}

// normalizeBaseURL trims and validates a site root URL.
func normalizeBaseURL(raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	pu, err := url.Parse(raw)
	if err != nil || (pu.Scheme != "http" && pu.Scheme != "https") || pu.Host == "" {
		return "", resp.Invalid("站点地址必须是 http(s):// 开头的根地址")
	}
	return raw, nil
}

func (s *Server) applyUpstreamInput(u *store.Upstream, in UpstreamInput) error {
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if n == "" || len(n) > 64 {
			return resp.Invalid("名称不能为空，且不超过 64 个字符")
		}
		u.Name = n
	}
	if in.Type != nil {
		if !upstreamTypes[*in.Type] {
			return resp.Invalid("未知的上游类型")
		}
		u.Type = *in.Type
	}
	if in.BaseURL != nil {
		raw, err := normalizeBaseURL(*in.BaseURL)
		if err != nil {
			return err
		}
		u.BaseURL = raw
	}
	if in.CredType != nil {
		if !credTypes[*in.CredType] {
			return resp.Invalid("未知的凭证类型")
		}
		u.CredType = *in.CredType
	}
	if in.PlatformUserID != nil {
		u.PlatformUserID = *in.PlatformUserID
	}
	var writeCred *Credential
	if in.Credential != nil && !in.Credential.empty() {
		writeCred = in.Credential
	}
	if in.Type != nil || in.CredType != nil || in.PlatformUserID != nil || in.Credential != nil {
		if err := checkSessionCred(u.Type, u.CredType, u.PlatformUserID, writeCred); err != nil {
			return err
		}
	}
	if in.Credential != nil {
		if writeCred == nil {
			u.Cred = nil
		} else {
			b, _ := json.Marshal(writeCred)
			sealed, err := s.Box.Seal(b)
			if err != nil {
				return err
			}
			u.Cred = sealed
		}
		u.CredUpdatedAt = time.Now().Unix()
	}
	if in.CurrencyRate != nil {
		r := *in.CurrencyRate
		if math.IsNaN(r) || math.IsInf(r, 0) || r <= 0 {
			return resp.Invalid("上游货币汇率必须大于 0")
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
			return resp.Invalid("适配器配置必须是 JSON")
		}
		u.AdapterConfig = *in.AdapterConfig
	}
	if u.Type == store.UpstreamGeneric && (in.AdapterConfig != nil || in.Type != nil) {
		if err := upstream.ValidateGenericConfig(u.AdapterConfig); err != nil {
			return resp.Invalid(err.Error())
		}
	}
	return nil
}

func saveErr(err error) error {
	if strings.Contains(err.Error(), "UNIQUE") {
		return resp.Conflict("上游名称已存在")
	}
	return err
}

func (s *Server) createUpstream(c *gin.Context) {
	var in UpstreamInput
	if !resp.BindJSON(c, &in) {
		return
	}
	if in.Name == nil || in.Type == nil || in.BaseURL == nil {
		resp.FailFrom(c, resp.Invalid("名称、类型和站点地址必填"))
		return
	}
	u := store.Upstream{CurrencyRate: 1}
	if err := s.applyUpstreamInput(&u, in); err != nil {
		resp.FailFrom(c, err)
		return
	}
	ctx := c.Request.Context()
	if err := s.Store.SaveUpstream(ctx, &u); err != nil {
		resp.FailFrom(c, saveErr(err))
		return
	}
	s.Audit.Record(ctx, store.AuditLog{Kind: store.AuditUpstream, Target: "upstream:" + strconv.Itoa(int(u.ID)), Action: "create", Summary: "新增上游 " + u.Name})
	resp.OK(c, s.view(u, nil, nil))
}

func (s *Server) updateUpstream(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var in UpstreamInput
	if !resp.BindJSON(c, &in) {
		return
	}
	ctx := c.Request.Context()
	u, err := s.Store.GetUpstream(ctx, id)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	if err := s.applyUpstreamInput(&u, in); err != nil {
		resp.FailFrom(c, err)
		return
	}
	if err := s.Store.SaveUpstream(ctx, &u); err != nil {
		resp.FailFrom(c, saveErr(err))
		return
	}
	s.Audit.Record(ctx, store.AuditLog{Kind: store.AuditUpstream, Target: "upstream:" + strconv.Itoa(int(u.ID)), Action: "update", Summary: "修改上游 " + u.Name})
	tk, ch, _ := s.Store.UpstreamCounts(ctx)
	resp.OK(c, s.view(u, tk, ch))
}

// deleteUpstream drafts the deletion of every channel of the upstream, then removes the
// upstream and its synced data. The channel deletions are applied from the draft flow.
func (s *Server) deleteUpstream(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	ctx := c.Request.Context()
	u, err := s.Store.GetUpstream(ctx, id)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	marked, err := s.Channels.MarkDeleteForUpstream(ctx, id)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	if err := s.Store.DeleteUpstream(ctx, id); err != nil {
		resp.FailFrom(c, err)
		return
	}
	s.Audit.Record(ctx, store.AuditLog{Kind: store.AuditUpstream, Target: "upstream:" + strconv.Itoa(int(id)), Action: "delete",
		Summary: "删除上游 " + u.Name + "，标记删除 " + strconv.Itoa(marked) + " 个渠道"})
	resp.OK(c, gin.H{"deleted_channels": marked})
}

func (s *Server) revealCredential(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	ctx := c.Request.Context()
	u, err := s.Store.GetUpstream(ctx, id)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	pt, err := s.Box.Open(u.Cred)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	var cred Credential
	if len(pt) > 0 {
		_ = json.Unmarshal(pt, &cred)
	}
	s.Audit.Record(ctx, store.AuditLog{Kind: store.AuditUpstream, Target: "upstream:" + strconv.Itoa(int(id)), Action: "reveal", Summary: "查看上游凭证 " + u.Name})
	resp.OK(c, cred)
}
