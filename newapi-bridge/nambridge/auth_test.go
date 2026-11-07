package nambridge

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func seedUser(t *testing.T, name string, role int, token string) *model.User {
	t.Helper()
	tok := token
	u := &model.User{
		Username: name, Password: "password-not-used", Role: role, Status: common.UserStatusEnabled,
		AccessToken: &tok, AffCode: "aff-" + name, Group: "default",
	}
	if err := model.DB.Create(u).Error; err != nil {
		t.Fatalf("seed %s: %v", name, err)
	}
	t.Cleanup(func() { model.DB.Unscoped().Delete(&model.User{}, u.Id) })
	return u
}

func TestAuth_onlyRootReachesNam(t *testing.T) {
	// Given: nam mounted on an engine with NewAPI-style cookie sessions
	t.Setenv("NAM_SECRET", "test-secret-for-nambridge")
	t.Setenv("NAM_DB_PATH", t.TempDir()+"/nam.db")
	t.Setenv("NAM_DISABLED", "")
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-session-secret"))))
	if err := Mount(engine); err != nil {
		t.Fatalf("mount: %v", err)
	}
	t.Cleanup(Stop)

	root := seedUser(t, "nb_root", common.RoleRootUser, "rootrootrootrootrootrootrootroot")
	admin := seedUser(t, "nb_admin", common.RoleAdminUser, "adminadminadminadminadminadmin12")
	user := seedUser(t, "nb_user", common.RoleCommonUser, "useruseruseruseruseruseruseruser")

	do := func(token string, id int) (int, string) {
		req := httptest.NewRequest(http.MethodGet, "/api/nam/upstreams", nil)
		req.Header.Set("Authorization", token)
		req.Header.Set("New-Api-User", itoa(id))
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}

	// When / Then: root passes
	if code, body := do(*root.AccessToken, root.Id); code != http.StatusOK || !jsonSuccessNotFalse(body) {
		t.Fatalf("root: %d %s", code, body)
	}
	// admin and normal user are rejected (NewAPI envelope: success=false)
	for name, u := range map[string]*model.User{"admin": admin, "user": user} {
		code, body := do(*u.AccessToken, u.Id)
		if code != http.StatusForbidden || jsonSuccessNotFalse(body) {
			t.Fatalf("%s must be rejected with 403, got %d %s", name, code, body)
		}
	}
	// anonymous is rejected
	req := httptest.NewRequest(http.MethodGet, "/api/nam/upstreams", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", rec.Code)
	}
}

func TestMount_skipsWithoutSecret_andKeepsEngineUsable(t *testing.T) {
	// Given: no NAM_SECRET
	t.Setenv("NAM_SECRET", "")
	t.Setenv("NAM_DISABLED", "")
	engine := gin.New()

	// When
	err := Mount(engine)

	// Then: no error, no nam routes
	if err != nil {
		t.Fatalf("mount without secret must not fail: %v", err)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/nam/upstreams", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when not mounted, got %d", rec.Code)
	}
}
