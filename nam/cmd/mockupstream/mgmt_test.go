package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

// authHeaders returns the raw NewAPI-style auth headers.
func authHeaders() map[string]string {
	return map[string]string{
		"Authorization": defaultAccessToken,
		"New-Api-User":  "1",
	}
}

func TestStatusPublic(t *testing.T) {
	s := newTestServer()
	rec := do(s, http.MethodGet, "/api/status", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Version           string `json:"version"`
			QuotaPerUnit      int    `json:"quota_per_unit"`
			SystemName        string `json:"system_name"`
			DisplayInCurrency bool   `json:"display_in_currency"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success || resp.Data.QuotaPerUnit != 500000 || resp.Data.SystemName != "mock-upstream" || !resp.Data.DisplayInCurrency {
		t.Fatalf("unexpected status payload: %+v", resp)
	}
}

func TestUserSelfAuth(t *testing.T) {
	s := newTestServer()

	unauth := do(s, http.MethodGet, "/api/user/self", "", nil)
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status = %d, want 401", unauth.Code)
	}

	rec := do(s, http.MethodGet, "/api/user/self", "", authHeaders())
	if rec.Code != http.StatusOK {
		t.Fatalf("auth status = %d, want 200", rec.Code)
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			ID        int    `json:"id"`
			Username  string `json:"username"`
			Role      int    `json:"role"`
			Status    int    `json:"status"`
			Quota     int    `json:"quota"`
			UsedQuota int    `json:"used_quota"`
			Group     string `json:"group"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success || resp.Data.ID != 1 || resp.Data.Username != "mock" || resp.Data.Quota != 2500000 || resp.Data.Group != "default" {
		t.Fatalf("unexpected self payload: %+v", resp)
	}
}

func TestBearerTokenAccepted(t *testing.T) {
	s := newTestServer()
	rec := do(s, http.MethodGet, "/api/user/self", "", map[string]string{
		"Authorization": "Bearer " + defaultAccessToken,
		"New-Api-User":  "1",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("bearer status = %d, want 200", rec.Code)
	}
}

func TestTokensList(t *testing.T) {
	s := newTestServer()
	rec := do(s, http.MethodGet, "/api/token/?p=1&page_size=10", "", authHeaders())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp struct {
		Data struct {
			Items []struct {
				ID    int    `json:"id"`
				Name  string `json:"name"`
				Key   string `json:"key"`
				Group string `json:"group"`
			} `json:"items"`
			Total    int `json:"total"`
			Page     int `json:"page"`
			PageSize int `json:"page_size"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Total != 3 || len(resp.Data.Items) != 3 {
		t.Fatalf("total/items = %d/%d, want 3/3", resp.Data.Total, len(resp.Data.Items))
	}
	groups := map[string]bool{}
	for _, it := range resp.Data.Items {
		groups[it.Group] = true
		if it.Key == "" {
			t.Fatalf("token %d missing masked key", it.ID)
		}
	}
	for _, g := range []string{"a", "b", "c"} {
		if !groups[g] {
			t.Fatalf("missing group %q in %+v", g, resp.Data.Items)
		}
	}
}

func TestCtlQuotaReflected(t *testing.T) {
	s := newTestServer()
	ctl := do(s, http.MethodPost, "/_ctl", `{"quota":123}`, nil)
	if ctl.Code != http.StatusOK {
		t.Fatalf("ctl status = %d", ctl.Code)
	}
	rec := do(s, http.MethodGet, "/api/user/self", "", authHeaders())
	var resp struct {
		Data struct {
			Quota int `json:"quota"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Quota != 123 {
		t.Fatalf("quota = %d, want 123", resp.Data.Quota)
	}
}

func TestGroupRatioReflected(t *testing.T) {
	s := newTestServer()
	do(s, http.MethodPost, "/_ctl", `{"group_ratio":{"x":2.5}}`, nil)

	groupsRec := do(s, http.MethodGet, "/api/user/self/groups", "", authHeaders())
	var groupsResp struct {
		Data map[string]struct {
			Ratio float64 `json:"ratio"`
			Desc  string  `json:"desc"`
		} `json:"data"`
	}
	if err := json.Unmarshal(groupsRec.Body.Bytes(), &groupsResp); err != nil {
		t.Fatalf("decode groups: %v", err)
	}
	if groupsResp.Data["x"].Ratio != 2.5 || groupsResp.Data["x"].Desc != "x" {
		t.Fatalf("group x = %+v, want ratio 2.5 desc x", groupsResp.Data["x"])
	}

	pricingRec := do(s, http.MethodGet, "/api/pricing", "", nil)
	var pricingResp struct {
		GroupRatio map[string]float64 `json:"group_ratio"`
	}
	if err := json.Unmarshal(pricingRec.Body.Bytes(), &pricingResp); err != nil {
		t.Fatalf("decode pricing: %v", err)
	}
	if pricingResp.GroupRatio["x"] != 2.5 {
		t.Fatalf("pricing group_ratio x = %v, want 2.5", pricingResp.GroupRatio["x"])
	}
}

func TestPricingShape(t *testing.T) {
	s := newTestServer()
	rec := do(s, http.MethodGet, "/api/pricing", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp struct {
		Success     bool               `json:"success"`
		Data        []map[string]any   `json:"data"`
		GroupRatio  map[string]float64 `json:"group_ratio"`
		UsableGroup map[string]string  `json:"usable_group"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success || len(resp.Data) != 2 {
		t.Fatalf("pricing data len = %d, want 2", len(resp.Data))
	}
	if resp.GroupRatio["a"] != 0.4 || resp.UsableGroup["a"] != "a" {
		t.Fatalf("unexpected group ratio/usable: %+v %+v", resp.GroupRatio, resp.UsableGroup)
	}
}

func TestTokenCreateThenList(t *testing.T) {
	s := newTestServer()
	rec := do(s, http.MethodPost, "/api/token/", `{"name":"new-tok","group":"a","remain_quota":5000,"unlimited_quota":false,"expired_time":-1}`, authHeaders())
	if rec.Code != http.StatusOK {
		t.Fatalf("create status = %d, want 200", rec.Code)
	}

	list := do(s, http.MethodGet, "/api/token/?p=1&page_size=10", "", authHeaders())
	var resp struct {
		Data struct {
			Items []struct {
				Name string `json:"name"`
			} `json:"items"`
			Total int `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Total != 4 || len(resp.Data.Items) != 4 {
		t.Fatalf("total/items = %d/%d, want 4/4", resp.Data.Total, len(resp.Data.Items))
	}
	if resp.Data.Items[3].Name != "new-tok" {
		t.Fatalf("last token name = %q, want new-tok", resp.Data.Items[3].Name)
	}
}

func TestTokenKeyByID(t *testing.T) {
	s := newTestServer()
	rec := do(s, http.MethodPost, "/api/token/1/key", "", authHeaders())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp struct {
		Data struct {
			Key string `json:"key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Key != "sk-mock-a" {
		t.Fatalf("key = %q, want sk-mock-a", resp.Data.Key)
	}
}

func TestNoticeChange(t *testing.T) {
	s := newTestServer()
	do(s, http.MethodPost, "/_ctl", `{"notice":"hello notice"}`, nil)
	rec := do(s, http.MethodGet, "/api/notice", "", nil)
	var resp struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data != "hello notice" {
		t.Fatalf("notice = %q, want hello notice", resp.Data)
	}
}

func TestCustomBalance(t *testing.T) {
	s := newTestServer()
	do(s, http.MethodPost, "/_ctl", `{"custom_balance":{"foo":"bar","n":7}}`, nil)

	rec := do(s, http.MethodGet, "/_custom/bal", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["foo"] != "bar" {
		t.Fatalf("foo = %v, want bar (body=%s)", resp["foo"], rec.Body.String())
	}
}

func TestSessionLoginFlow(t *testing.T) {
	s := newTestServer()
	login := do(s, http.MethodPost, "/api/user/login", `{"username":"mock","password":"mock-pass"}`, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", login.Code)
	}
	var loginResp struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	if !loginResp.Success {
		t.Fatalf("login success = false")
	}

	cookie := login.Header().Get("Set-Cookie")
	if cookie == "" {
		t.Fatalf("missing session cookie")
	}

	rec := do(s, http.MethodGet, "/api/user/self", "", map[string]string{
		"Cookie":       "session=mock-session",
		"New-Api-User": "1",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("session self status = %d, want 200 (cookie=%q)", rec.Code, cookie)
	}
}

func TestResetRestoresDefaults(t *testing.T) {
	s := newTestServer()
	do(s, http.MethodPost, "/_ctl", `{"quota":1,"notice":"changed","tokens":[{"id":9,"name":"only","key":"sk-x","group":"a","status":1}]}`, nil)
	do(s, http.MethodPost, "/_ctl", `{"reset":true}`, nil)

	self := do(s, http.MethodGet, "/api/user/self", "", authHeaders())
	var selfResp struct {
		Data struct {
			Quota int `json:"quota"`
		} `json:"data"`
	}
	if err := json.Unmarshal(self.Body.Bytes(), &selfResp); err != nil {
		t.Fatalf("decode self: %v", err)
	}
	if selfResp.Data.Quota != 2500000 {
		t.Fatalf("quota after reset = %d, want 2500000", selfResp.Data.Quota)
	}

	notice := do(s, http.MethodGet, "/api/notice", "", nil)
	var noticeResp struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(notice.Body.Bytes(), &noticeResp); err != nil {
		t.Fatalf("decode notice: %v", err)
	}
	if noticeResp.Data != "mock notice v1" {
		t.Fatalf("notice after reset = %q, want mock notice v1", noticeResp.Data)
	}

	list := do(s, http.MethodGet, "/api/token/?p=1&page_size=10", "", authHeaders())
	var listResp struct {
		Data struct {
			Total int `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if listResp.Data.Total != 3 {
		t.Fatalf("token total after reset = %d, want 3", listResp.Data.Total)
	}
}

func TestManagementCallsRecorded(t *testing.T) {
	s := newTestServer()
	do(s, http.MethodGet, "/api/status", "", nil)
	do(s, http.MethodGet, "/_custom/bal", "", nil)
	calls := s.snapshotCalls()
	if len(calls) != 2 {
		t.Fatalf("recorded %d calls, want 2", len(calls))
	}
	if calls[0].Path != "/api/status" || calls[1].Path != "/_custom/bal" {
		t.Fatalf("unexpected recorded paths: %+v", calls)
	}
}
