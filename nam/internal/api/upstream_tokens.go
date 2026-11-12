package api

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/internal/upstream"
)

// keyHint mirrors upstream's display form: first 5 + … + last 4.
func keyHint(key string) string {
	r := []rune(key)
	switch {
	case len(r) <= 2:
		return "…"
	case len(r) <= 9:
		return string(r[:2]) + "…"
	}
	return string(r[:5]) + "…" + string(r[len(r)-4:])
}

// GET /upstreams/:id/tokens — never includes the key, only key_hint.
func (s *Server) listTokens(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var ts []store.UpstreamToken
	if err := s.Store.DB.WithContext(c.Request.Context()).Where("upstream_id = ?", id).Order("id").Find(&ts).Error; err != nil {
		resp.FailFrom(c, err)
		return
	}
	ids := make([]uint, len(ts))
	for i := range ts {
		ids[i] = ts[i].ID
	}
	counts, err := s.tokenChannelCounts(c.Request.Context(), ids)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	out := make([]tokenView, len(ts))
	for i, t := range ts {
		out[i] = tokenView{UpstreamToken: t, ChannelCount: counts[t.ID]}
	}
	resp.OK(c, out)
}

// tokenView is a token plus the number of channels (applied or drafted) built on it.
type tokenView struct {
	store.UpstreamToken
	ChannelCount int `json:"channel_count"`
}

// tokenChannelCounts returns the channel_ext reference count per token id in one grouped query.
func (s *Server) tokenChannelCounts(ctx context.Context, ids []uint) (map[uint]int, error) {
	counts := map[uint]int{}
	if len(ids) == 0 {
		return counts, nil
	}
	type row struct {
		ID uint
		N  int
	}
	var rows []row
	if err := s.Store.DB.WithContext(ctx).Model(&store.ChannelExt{}).Where("token_id IN ?", ids).
		Select("token_id as id, count(*) as n").Group("token_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		counts[r.ID] = r.N
	}
	return counts, nil
}

// POST /upstreams/:id/tokens {name, group, key}: record a token created by hand.
func (s *Server) addManualToken(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var in struct {
		Name  string `json:"name"`
		Group string `json:"group"`
		Key   string `json:"key"`
	}
	if !resp.BindJSON(c, &in) {
		return
	}
	in.Name, in.Group, in.Key = strings.TrimSpace(in.Name), strings.TrimSpace(in.Group), strings.TrimSpace(in.Key)
	if in.Name == "" || len(in.Name) > 128 || in.Key == "" || len(in.Group) > 64 {
		resp.FailFrom(c, resp.Invalid("名称和 key 必填，名称不超过 128、分组不超过 64 个字符"))
		return
	}
	ctx := c.Request.Context()
	u, err := s.Store.GetUpstream(ctx, id)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	sealed, err := s.Box.Seal([]byte(in.Key))
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	t := store.UpstreamToken{UpstreamID: id, Name: in.Name, Group: in.Group, Key: sealed, KeyHint: keyHint(in.Key),
		Status: 1, Manual: true, UpdatedAt: time.Now().Unix()}
	if err := s.Store.DB.WithContext(ctx).Create(&t).Error; err != nil {
		resp.FailFrom(c, err)
		return
	}
	s.Audit.Record(ctx, store.AuditLog{Kind: store.AuditUpstream, Target: "upstream:" + strconv.Itoa(int(id)), Action: "add_token",
		Summary: fmt.Sprintf("在上游 %s 手动录入令牌 %s", u.Name, t.Name)})
	resp.OK(c, t)
}

// POST /upstreams/:id/tokens/remote {name, group, quota_usd}: create the token on the upstream.
func (s *Server) createRemoteToken(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var in struct {
		Name     string   `json:"name"`
		Group    string   `json:"group"`
		QuotaUSD *float64 `json:"quota_usd"`
	}
	if !resp.BindJSON(c, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len(in.Name) > 128 {
		resp.FailFrom(c, resp.Invalid("令牌名称不能为空，且不超过 128 个字符"))
		return
	}
	if in.QuotaUSD != nil && *in.QuotaUSD <= 0 {
		resp.FailFrom(c, resp.Invalid("额度必须大于 0，留空表示不限额"))
		return
	}
	if _, err := s.Store.GetUpstream(c.Request.Context(), id); err != nil {
		resp.FailFrom(c, err)
		return
	}
	ctx, cancel := withTimeout(c, time.Minute)
	defer cancel()
	t, err := s.Syncer.CreateToken(ctx, id, upstream.TokenCreate{Name: in.Name, Group: in.Group, QuotaUSD: in.QuotaUSD})
	if err != nil {
		if errIsUnsupported(err) {
			resp.FailFrom(c, resp.Invalid("该上游类型不支持自动创建令牌，请手动录入"))
			return
		}
		resp.FailFrom(c, resp.Upstream(err))
		return
	}
	resp.OK(c, t)
}

// DELETE /upstreams/:id/tokens/:tid — refused while a channel (applied or drafted) uses it.
func (s *Server) deleteToken(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	tid, okTID := parseUintParam(c, "tid")
	if !okTID {
		return
	}
	ctx := c.Request.Context()
	var t store.UpstreamToken
	if err := s.Store.DB.WithContext(ctx).Where("id = ? AND upstream_id = ?", tid, id).First(&t).Error; err != nil {
		resp.FailFrom(c, store.ErrNotFound)
		return
	}
	var refs int64
	if err := s.Store.DB.WithContext(ctx).Model(&store.ChannelExt{}).Where("token_id = ?", tid).Count(&refs).Error; err != nil {
		resp.FailFrom(c, err)
		return
	}
	if refs > 0 {
		resp.FailFrom(c, resp.Conflict(fmt.Sprintf("令牌 %s 仍被 %d 个渠道使用，不能删除", t.Name, refs)))
		return
	}
	if err := s.Store.DB.WithContext(ctx).Delete(&store.UpstreamToken{}, tid).Error; err != nil {
		resp.FailFrom(c, err)
		return
	}
	s.Audit.Record(ctx, store.AuditLog{Kind: store.AuditUpstream, Target: "upstream:" + strconv.Itoa(int(id)), Action: "delete_token",
		Summary: "删除令牌 " + t.Name})
	resp.OK(c, nil)
}
