package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

// userAuth takes the user id from the X-Uid header so one test can act as several users
// (the portstest fake always signs in as user 1).
type userAuth struct{}

func (userAuth) Mode() string                      { return "test" }
func (userAuth) Middleware() gin.HandlerFunc       { return func(c *gin.Context) { c.Next() } }
func (userAuth) SecureMiddleware() gin.HandlerFunc { return func(c *gin.Context) { c.Next() } }
func (userAuth) Identity(c *gin.Context) (ports.Identity, bool) {
	uid, err := strconv.Atoi(c.GetHeader("X-Uid"))
	if err != nil {
		return ports.Identity{}, false
	}
	return ports.Identity{UserID: uid, Username: "u", Role: 100}, true
}

func TestUILayoutPerUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, err := store.Open(filepath.Join(t.TempDir(), "nam.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	s := &Server{Deps{Store: st, Ports: ports.Ports{Auth: userAuth{}}}}
	r := gin.New()
	r.GET("/ui/layout", s.getUILayout)
	r.PUT("/ui/layout", s.putUILayout)

	call := func(method, uid, body string) (int, string) {
		req := httptest.NewRequest(method, "/ui/layout", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if uid != "" {
			req.Header.Set("X-Uid", uid)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code, w.Body.String()
	}

	if code, _ := call("PUT", "1", `{"layout":{"who":"one"}}`); code != http.StatusOK {
		t.Fatalf("user 1 save: %d", code)
	}
	if code, body := call("GET", "1", ""); code != 200 || !strings.Contains(body, `"layout":{"who":"one"}`) {
		t.Fatalf("user 1 read: %d %s", code, body)
	}
	// user 2 sees nothing, then saves without touching user 1
	if code, body := call("GET", "2", ""); code != 200 || !strings.Contains(body, `"layout":null`) || !strings.Contains(body, `"saved_at":0`) {
		t.Fatalf("user 2 read: %d %s", code, body)
	}
	if code, _ := call("PUT", "2", `{"layout":{"who":"two"}}`); code != 200 {
		t.Fatalf("user 2 save: %d", code)
	}
	if _, body := call("GET", "1", ""); !strings.Contains(body, `"who":"one"`) {
		t.Fatalf("user 1 changed by user 2: %s", body)
	}
	if _, body := call("GET", "2", ""); !strings.Contains(body, `"who":"two"`) {
		t.Fatalf("user 2 read: %s", body)
	}
	for _, k := range []string{store.UILayoutKey(1), store.UILayoutKey(2)} {
		if _, ok, err := st.GetKV(t.Context(), k); err != nil || !ok {
			t.Fatalf("key %s missing: %v", k, err)
		}
	}
	if store.UILayoutKey(7) != "ui_layout.7" {
		t.Fatalf("key format: %s", store.UILayoutKey(7))
	}

	// no identity, or a non-positive user id: 401 for both verbs
	for _, uid := range []string{"", "0", "-3", "x"} {
		for _, method := range []string{"GET", "PUT"} {
			if code, _ := call(method, uid, `{"layout":{}}`); code != http.StatusUnauthorized {
				t.Errorf("%s uid=%q: want 401, got %d", method, uid, code)
			}
		}
	}
}
