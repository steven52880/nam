package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/resp"
	"github.com/steven52880/nam/internal/store"
)

// maxLayoutBytes caps one PUT /ui/layout request body.
const maxLayoutBytes = 256 << 10

// layoutDoc is the stored value (and the GET response): the layout stays opaque to the backend.
type layoutDoc struct {
	SavedAt int64           `json:"saved_at"`
	Layout  json.RawMessage `json:"layout"` // nil encodes as null
}

// layoutUser returns the operator's user id, or writes a 401.
func (s *Server) layoutUser(c *gin.Context) (int, bool) {
	id, ok := s.Ports.Auth.Identity(c)
	if !ok || id.UserID <= 0 {
		resp.Fail(c, http.StatusUnauthorized, errors.New("未登录"))
		return 0, false
	}
	return id.UserID, true
}

// GET /ui/layout → {layout: {...}|null, saved_at}
func (s *Server) getUILayout(c *gin.Context) {
	uid, ok := s.layoutUser(c)
	if !ok {
		return
	}
	raw, found, err := s.Store.GetKV(c.Request.Context(), store.UILayoutKey(uid))
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	out := layoutDoc{}
	if found {
		// an unreadable record is treated as "never saved": the next PUT overwrites it
		if json.Unmarshal([]byte(raw), &out) != nil || bytes.Equal(bytes.TrimSpace(out.Layout), []byte("null")) {
			out = layoutDoc{}
		}
	}
	resp.OK(c, out)
}

// PUT /ui/layout {layout: {...}} → {saved_at}. Not audited: it is per-user UI state.
func (s *Server) putUILayout(c *gin.Context) {
	uid, ok := s.layoutUser(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxLayoutBytes)
	// read the whole body so the size cap covers everything, then Unmarshal rejects trailing content
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			resp.Fail(c, http.StatusBadRequest, resp.Invalid("布局太大"))
			return
		}
		resp.Fail(c, http.StatusBadRequest, resp.Invalid("读取请求体失败："+err.Error()))
		return
	}
	var in struct {
		Layout json.RawMessage `json:"layout"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		resp.Fail(c, http.StatusBadRequest, resp.Invalid("请求体不是合法 JSON："+err.Error()))
		return
	}
	trimmed := bytes.TrimSpace(in.Layout)
	if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(trimmed) {
		resp.Fail(c, http.StatusBadRequest, resp.Invalid("layout 必须是 JSON 对象"))
		return
	}
	doc := layoutDoc{SavedAt: time.Now().Unix(), Layout: trimmed}
	b, err := json.Marshal(doc)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	if err := s.Store.SetKV(c.Request.Context(), store.UILayoutKey(uid), string(b)); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"saved_at": doc.SavedAt})
}
