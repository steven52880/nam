package core_test

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/glebarez/go-sqlite"
	"github.com/steven52880/nam/core"
	"github.com/steven52880/nam/ports/portstest"
)

func newCore(t *testing.T) (*core.Core, http.Handler, string) {
	t.Helper()
	db := filepath.Join(t.TempDir(), "nam.db")
	c, err := core.New(core.Config{DBPath: db, Secret: "test-secret"}, portstest.NewFake().Ports())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Stop)
	return c, c.Handler(), db
}

func do(h http.Handler, method, path, body, role string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if role != "" {
		req.Header.Set("X-Fake-Role", role)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestAuthGate(t *testing.T) {
	_, h, _ := newCore(t)
	if w := do(h, "GET", "/api/nam/upstreams", "", ""); w.Code != 401 {
		t.Fatalf("anon: %d", w.Code)
	}
	if w := do(h, "GET", "/api/nam/upstreams", "", "10"); w.Code != 403 {
		t.Fatalf("admin: %d", w.Code)
	}
	if w := do(h, "GET", "/api/nam/upstreams", "", "1"); w.Code != 403 {
		t.Fatalf("user: %d", w.Code)
	}
	if w := do(h, "GET", "/api/nam/upstreams", "", "100"); w.Code != 200 {
		t.Fatalf("root: %d %s", w.Code, w.Body)
	}
	if w := do(h, "GET", "/api/nam/ping", "", ""); w.Code != 200 {
		t.Fatalf("ping: %d", w.Code)
	}
}

// M1 QA step 4: credentials are not stored in plaintext.
func TestCredentialEncrypted(t *testing.T) {
	c, h, dbPath := newCore(t)
	body := `{"name":"mock-a","type":"newapi","base_url":"http://127.0.0.1:9000/","cred_type":"sk_key","credential":{"key":"sk-plaintext-test"},"currency_rate":0.5}`
	w := do(h, "POST", "/api/nam/upstreams", body, "100")
	if w.Code != 200 {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	if strings.Contains(w.Body.String(), "sk-plaintext-test") {
		t.Fatal("response leaks credential")
	}
	var env struct {
		Data struct {
			ID            uint   `json:"id"`
			BaseURL       string `json:"base_url"`
			HasCredential bool   `json:"has_credential"`
		}
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	if env.Data.BaseURL != "http://127.0.0.1:9000" || !env.Data.HasCredential {
		t.Fatalf("bad view: %+v", env.Data)
	}
	c.Stop()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var h1 string
	if err := db.QueryRow("select hex(cred) from upstreams").Scan(&h1); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToUpper(h1), strings.ToUpper(hex.EncodeToString([]byte("sk-plaintext-test")))) {
		t.Fatal("plaintext found in db")
	}
}

func TestUpstreamValidationAndAudit(t *testing.T) {
	_, h, _ := newCore(t)
	if w := do(h, "POST", "/api/nam/upstreams", `{"name":"x","type":"newapi","base_url":"ftp://a"}`, "100"); w.Code != 400 {
		t.Fatalf("bad url: %d", w.Code)
	}
	if w := do(h, "POST", "/api/nam/upstreams", `{"name":"x","type":"bogus","base_url":"https://a.com"}`, "100"); w.Code != 400 {
		t.Fatalf("bad type: %d", w.Code)
	}
	if w := do(h, "POST", "/api/nam/upstreams", `{"name":"x","type":"newapi","base_url":"https://a.com","currency_rate":0}`, "100"); w.Code != 400 {
		t.Fatalf("bad rate: %d", w.Code)
	}
	do(h, "POST", "/api/nam/upstreams", `{"name":"x","type":"newapi","base_url":"https://a.com"}`, "100")
	if w := do(h, "POST", "/api/nam/upstreams", `{"name":"x","type":"newapi","base_url":"https://b.com"}`, "100"); w.Code != 409 {
		t.Fatalf("dup: %d", w.Code)
	}
	w := do(h, "GET", "/api/nam/audit", "", "100")
	if !strings.Contains(w.Body.String(), "新增上游 x") || !strings.Contains(w.Body.String(), `"operator":"root"`) {
		t.Fatalf("audit missing: %s", w.Body)
	}
}

func TestSPAAndJobs(t *testing.T) {
	_, h, _ := newCore(t)
	for _, p := range []string{"/nam/", "/nam/channels/12"} {
		w := do(h, "GET", p, "", "")
		if w.Code != 200 || !strings.Contains(w.Body.String(), "<html") {
			t.Fatalf("%s: %d", p, w.Code)
		}
	}
	w := do(h, "GET", "/api/nam/jobs", "", "100")
	if !strings.Contains(w.Body.String(), "pull_logs") {
		t.Fatalf("jobs: %s", w.Body)
	}
	if w := do(h, "POST", "/api/nam/jobs/nope", "", "100"); w.Code != 404 {
		t.Fatalf("unknown job: %d", w.Code)
	}
	if w := do(h, "PUT", "/api/nam/settings", `{"site_rate":"1"}`, "100"); w.Code != 400 {
		t.Fatalf("site_rate must be rejected: %d", w.Code)
	}
	if w := do(h, "PUT", "/api/nam/settings", `{"job_interval.balance":"45m"}`, "100"); w.Code != 200 || !strings.Contains(w.Body.String(), "45m") {
		t.Fatalf("interval: %d %s", w.Code, w.Body)
	}
	if w := do(h, "GET", "/api/nam/newapi/channels", "", "100"); w.Code != 200 || !strings.Contains(w.Body.String(), `"total":0`) {
		t.Fatalf("channels: %s", w.Body)
	}
}
