package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultQuota       = 2500000
	quotaPerUnit       = 500000
	defaultAccessToken = "mock-access-token"
	defaultUserID      = 1
	authFailMessage    = "无权进行此操作，未登录且未提供 access token"
)

// tokenState is one management token entry.
type tokenState struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Key            string `json:"key"`
	Group          string `json:"group"`
	Status         int    `json:"status"`
	RemainQuota    int    `json:"remain_quota"`
	UnlimitedQuota bool   `json:"unlimited_quota"`
	UsedQuota      int    `json:"used_quota"`
	CreatedTime    int64  `json:"created_time"`
	ExpiredTime    int64  `json:"expired_time"`
}

// resetMgmt restores all management state to its defaults. Caller holds the lock.
func (s *server) resetMgmt() {
	s.quota = defaultQuota
	s.accessToken = defaultAccessToken
	s.userID = defaultUserID
	s.username = "mock"
	s.password = "mock-pass"
	s.notice = "mock notice v1"
	s.groupRatio = map[string]float64{"a": 0.4, "b": 0.5, "c": 1}
	s.modelRatio = map[string]float64{"test-model": 1.5, "test-model-2": 0.5}
	s.completionRatio = map[string]float64{"test-model": 5, "test-model-2": 4}
	s.tokens = []tokenState{
		{ID: 1, Name: "tok-a", Key: "sk-mock-a", Group: "a", Status: 1, RemainQuota: 1000000, ExpiredTime: -1},
		{ID: 2, Name: "tok-b", Key: "sk-mock-b", Group: "b", Status: 1, RemainQuota: 1000000, ExpiredTime: -1},
		{ID: 3, Name: "tok-c", Key: "sk-mock-c", Group: "c", Status: 1, RemainQuota: 1000000, ExpiredTime: -1},
	}
	s.nextTokenID = 4
	s.customBalance = json.RawMessage(`{"data":{"left":12345}}`)
}

func isManagementPath(p string) bool {
	return strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/_custom/")
}

func writeSuccess(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "", "data": data})
}

func writeFail(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"success": false, "message": msg})
}

func (s *server) handleManagement(w http.ResponseWriter, r *http.Request, body []byte) {
	p := r.URL.Path

	// Public endpoints and session login need no access token.
	switch {
	case p == "/api/status" && r.Method == http.MethodGet:
		s.handleStatus(w)
		return
	case p == "/api/pricing" && r.Method == http.MethodGet:
		s.handlePricing(w)
		return
	case p == "/api/notice" && r.Method == http.MethodGet:
		s.handleNotice(w)
		return
	case p == "/api/user/login" && r.Method == http.MethodPost:
		s.handleLogin(w, body)
		return
	case p == "/_custom/bal" && r.Method == http.MethodGet:
		s.handleCustomBal(w)
		return
	}

	if !s.authorized(r) {
		writeFail(w, http.StatusUnauthorized, authFailMessage)
		return
	}

	switch {
	case p == "/api/user/self" && r.Method == http.MethodGet:
		s.handleUserSelf(w)
	case p == "/api/user/self/groups" && r.Method == http.MethodGet:
		s.handleUserGroups(w)
	case (p == "/api/token/" || p == "/api/token") && r.Method == http.MethodGet:
		s.handleTokenList(w, r)
	case (p == "/api/token/" || p == "/api/token") && r.Method == http.MethodPost:
		s.handleTokenCreate(w, body)
	case strings.HasPrefix(p, "/api/token/") && strings.HasSuffix(p, "/key") && r.Method == http.MethodPost:
		s.handleTokenKey(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": map[string]any{"message": "not found", "type": "mock"},
		})
	}
}

// authorized reports whether the request may access a protected management
// endpoint. Caller must NOT hold the lock.
func (s *server) authorized(r *http.Request) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.authorizedLocked(r)
}

func (s *server) authorizedLocked(r *http.Request) bool {
	wantUser := strconv.Itoa(s.userID)
	if c, err := r.Cookie("session"); err == nil && c.Value == "mock-session" {
		return r.Header.Get("New-Api-User") == wantUser
	}
	auth := r.Header.Get("Authorization")
	tok := auth
	if strings.HasPrefix(auth, "Bearer ") || strings.HasPrefix(auth, "bearer ") {
		tok = strings.TrimSpace(auth[7:])
	}
	if tok == "" || tok != s.accessToken {
		return false
	}
	return r.Header.Get("New-Api-User") == wantUser
}

func (s *server) handleStatus(w http.ResponseWriter) {
	writeSuccess(w, map[string]any{
		"version":             "mock-1.0",
		"quota_per_unit":      quotaPerUnit,
		"system_name":         "mock-upstream",
		"display_in_currency": true,
	})
}

func (s *server) handleUserSelf(w http.ResponseWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeSuccess(w, map[string]any{
		"id":         s.userID,
		"username":   s.username,
		"role":       1,
		"status":     1,
		"quota":      s.quota,
		"used_quota": 0,
		"group":      "default",
	})
}

func (s *server) handleUserGroups(w http.ResponseWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	groups := make(map[string]any, len(s.groupRatio))
	for name, ratio := range s.groupRatio {
		groups[name] = map[string]any{"ratio": ratio, "desc": name}
	}
	writeSuccess(w, groups)
}

func (s *server) handleNotice(w http.ResponseWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeSuccess(w, s.notice)
}

func (s *server) handleLogin(w http.ResponseWriter, body []byte) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	_ = json.Unmarshal(body, &req)

	s.mu.Lock()
	ok := req.Username == s.username && req.Password == s.password
	uid := s.userID
	uname := s.username
	s.mu.Unlock()

	if !ok {
		writeFail(w, http.StatusOK, "用户名或密码错误")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "mock-session", Path: "/"})
	writeSuccess(w, map[string]any{"id": uid, "username": uname, "role": 1})
}

func (s *server) handleTokenList(w http.ResponseWriter, r *http.Request) {
	page := atoiDefault(r.URL.Query().Get("p"), 1)
	if page < 1 {
		page = 1
	}
	pageSize := atoiDefault(r.URL.Query().Get("page_size"), 10)
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	s.mu.Lock()
	total := len(s.tokens)
	items := make([]map[string]any, 0)
	start := (page - 1) * pageSize
	if start < total {
		end := start + pageSize
		if end > total {
			end = total
		}
		for _, t := range s.tokens[start:end] {
			items = append(items, tokenJSON(t))
		}
	}
	s.mu.Unlock()

	writeSuccess(w, map[string]any{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (s *server) handleTokenCreate(w http.ResponseWriter, body []byte) {
	var req struct {
		Name           string `json:"name"`
		Group          string `json:"group"`
		RemainQuota    int    `json:"remain_quota"`
		UnlimitedQuota bool   `json:"unlimited_quota"`
		ExpiredTime    int64  `json:"expired_time"`
	}
	_ = json.Unmarshal(body, &req)

	s.mu.Lock()
	id := s.nextTokenID
	if id < 1 {
		id = 1
	}
	expired := req.ExpiredTime
	if expired == 0 {
		expired = -1
	}
	s.tokens = append(s.tokens, tokenState{
		ID:             id,
		Name:           req.Name,
		Key:            fmt.Sprintf("sk-mock-%d", id),
		Group:          req.Group,
		Status:         1,
		RemainQuota:    req.RemainQuota,
		UnlimitedQuota: req.UnlimitedQuota,
		CreatedTime:    time.Now().Unix(),
		ExpiredTime:    expired,
	})
	s.nextTokenID = id + 1
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": ""})
}

func (s *server) handleTokenKey(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/token/"), "/key")
	id, err := strconv.Atoi(strings.Trim(idStr, "/"))
	if err != nil {
		writeFail(w, http.StatusOK, "无效的参数")
		return
	}

	s.mu.Lock()
	key := ""
	found := false
	for _, t := range s.tokens {
		if t.ID == id {
			key = t.Key
			found = true
			break
		}
	}
	s.mu.Unlock()

	if !found {
		writeFail(w, http.StatusOK, "令牌不存在")
		return
	}
	writeSuccess(w, map[string]any{"key": key})
}

func (s *server) handleCustomBal(w http.ResponseWriter) {
	s.mu.Lock()
	data := append(json.RawMessage(nil), s.customBalance...)
	s.mu.Unlock()

	if len(data) == 0 {
		data = json.RawMessage("null")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *server) handlePricing(w http.ResponseWriter) {
	s.mu.Lock()
	groupRatio := copyFloatMap(s.groupRatio)
	modelRatio := copyFloatMap(s.modelRatio)
	completionRatio := copyFloatMap(s.completionRatio)
	s.mu.Unlock()

	models := sortedKeys(modelRatio)
	groups := sortedKeys(groupRatio)

	data := make([]map[string]any, 0, len(models))
	for _, m := range models {
		data = append(data, map[string]any{
			"model_name":               m,
			"quota_type":               0,
			"model_ratio":              modelRatio[m],
			"model_price":              0,
			"completion_ratio":         completionRatio[m],
			"enable_groups":            groups,
			"supported_endpoint_types": []string{},
		})
	}

	usable := make(map[string]string, len(groupRatio))
	for g := range groupRatio {
		usable[g] = g
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":            true,
		"data":               data,
		"vendors":            []any{},
		"group_ratio":        groupRatio,
		"usable_group":       usable,
		"supported_endpoint": map[string]any{},
		"auto_groups":        []any{},
		"pricing_version":    "mock",
	})
}

func tokenJSON(t tokenState) map[string]any {
	return map[string]any{
		"id":              t.ID,
		"name":            t.Name,
		"key":             maskKey(t.Key),
		"group":           t.Group,
		"status":          t.Status,
		"remain_quota":    t.RemainQuota,
		"unlimited_quota": t.UnlimitedQuota,
		"used_quota":      t.UsedQuota,
		"created_time":    t.CreatedTime,
		"expired_time":    t.ExpiredTime,
	}
}

// maskKey mirrors NewAPI's model.MaskTokenKey.
func maskKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 4 {
		return strings.Repeat("*", len(key))
	}
	if len(key) <= 8 {
		return key[:2] + "****" + key[len(key)-2:]
	}
	return key[:4] + "**********" + key[len(key)-4:]
}

func copyFloatMap(in map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func maxTokenID(tokens []tokenState) int {
	max := 0
	for _, t := range tokens {
		if t.ID > max {
			max = t.ID
		}
	}
	return max
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
