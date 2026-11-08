package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/steven52880/nam/internal/store"
)

type fakeNewAPI struct {
	mu       sync.Mutex
	tokens   []map[string]any
	logins   int
	lastAuth string
	lastUser string
	cookieOK bool
}

func envelope(w http.ResponseWriter, data any) {
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "", "data": data})
}

func (f *fakeNewAPI) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, _ *http.Request) {
		envelope(w, map[string]any{"quota_per_unit": 500000, "version": "v0.9.1"})
	})
	mux.HandleFunc("/api/user/login", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		f.logins++
		f.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc", Path: "/"})
		envelope(w, map[string]any{"id": 42})
	})
	authed := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			f.lastAuth, f.lastUser = r.Header.Get("Authorization"), r.Header.Get("New-Api-User")
			c, err := r.Cookie("session")
			f.cookieOK = err == nil && c.Value == "abc"
			f.mu.Unlock()
			if r.Header.Get("Authorization") != "tok" && !f.cookieOK {
				http.Error(w, `{"success":false,"message":"无权进行此操作，未登录"}`, http.StatusUnauthorized)
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("/api/user/self", authed(func(w http.ResponseWriter, _ *http.Request) {
		envelope(w, map[string]any{"id": 42, "quota": 2500000})
	}))
	mux.HandleFunc("/api/user/self/groups", authed(func(w http.ResponseWriter, _ *http.Request) {
		envelope(w, map[string]any{"a": map[string]any{"ratio": 0.4, "desc": "A组"}, "auto": map[string]any{"ratio": "自动", "desc": "x"}})
	}))
	mux.HandleFunc("/api/notice", authed(func(w http.ResponseWriter, _ *http.Request) { envelope(w, " 公告内容 ") }))
	mux.HandleFunc("/api/pricing", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true, "group_ratio": map[string]any{"a": 0.4},
			"data": []map[string]any{
				{"model_name": "m1", "quota_type": 0, "model_ratio": 1.5, "completion_ratio": 5, "cache_ratio": 0.1, "enable_groups": []string{"a"}},
				{"model_name": "m2", "quota_type": 1, "model_price": 0.05, "enable_groups": []string{}},
				{"model_name": "m3", "quota_type": 0, "model_ratio": 1, "completion_ratio": 1, "enable_groups": []string{"zzz"}},
			},
		})
	})
	mux.HandleFunc("/api/token/", authed(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Method == http.MethodPost {
			var in map[string]any
			_ = json.NewDecoder(r.Body).Decode(&in)
			f.tokens = append(f.tokens, map[string]any{
				"id": len(f.tokens) + 1, "name": in["name"], "group": in["group"], "status": 1,
				"remain_quota": in["remain_quota"], "unlimited_quota": in["unlimited_quota"], "key": "ab**cd",
			})
			envelope(w, nil)
			return
		}
		p, _ := strconv.Atoi(r.URL.Query().Get("p"))
		ps, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
		lo, hi := (p-1)*ps, min(p*ps, len(f.tokens))
		items := []map[string]any{}
		if lo < len(f.tokens) {
			items = f.tokens[lo:hi]
		}
		envelope(w, map[string]any{"items": items, "total": len(f.tokens), "page": p})
	}))
	mux.HandleFunc("POST /api/token/{id}/key", authed(func(w http.ResponseWriter, r *http.Request) {
		envelope(w, map[string]any{"key": "full" + r.PathValue("id")})
	}))
	return mux
}

func newSite(url string) Site {
	return Site{Type: store.UpstreamNewAPI, BaseURL: url, CredType: store.CredAccessToken, Cred: Credential{AccessToken: "tok"}, UserID: 42}
}

func TestNewAPIProbe(t *testing.T) {
	srv := httptest.NewServer((&fakeNewAPI{}).handler())
	defer srv.Close()
	r, err := newAPIAdapter{}.Probe(context.Background(), Site{BaseURL: srv.URL})
	if err != nil || r.Type != "newapi" || r.Version != "v0.9.1" || r.Evidence != "/api/status 返回 quota_per_unit=500000（newapi）" {
		t.Fatalf("got %+v err=%v", r, err)
	}
	empty := httptest.NewServer(http.NotFoundHandler())
	defer empty.Close()
	if _, err := (newAPIAdapter{}).Probe(context.Background(), Site{BaseURL: empty.URL}); err == nil {
		t.Fatal("expected probe error")
	}
}

func TestNewAPIVerifyAuthRejected(t *testing.T) {
	srv := httptest.NewServer((&fakeNewAPI{}).handler())
	defer srv.Close()
	s := newSite(srv.URL)
	s.Cred.AccessToken = "bad"
	if err := (newAPIAdapter{}).Verify(context.Background(), s); !errIsAuth(err) {
		t.Fatalf("want ErrAuth, got %v", err)
	}
	if err := (newAPIAdapter{}).Verify(context.Background(), newSite(srv.URL)); err != nil {
		t.Fatal(err)
	}
}

func TestNewAPIBalance(t *testing.T) {
	f := &fakeNewAPI{}
	srv := httptest.NewServer(f.handler())
	defer srv.Close()
	b, err := newAPIAdapter{}.Balance(context.Background(), newSite(srv.URL))
	if err != nil || b.USD != 5.0 || b.Unlimited {
		t.Fatalf("got %+v err=%v", b, err)
	}
	if f.lastAuth != "tok" || f.lastUser != "42" {
		t.Fatalf("headers auth=%q user=%q", f.lastAuth, f.lastUser)
	}
}

func TestNewAPIListTokensPaginatesAndFillsKeys(t *testing.T) {
	f := &fakeNewAPI{}
	for i := 1; i <= 150; i++ {
		f.tokens = append(f.tokens, map[string]any{"id": i, "name": fmt.Sprintf("t%d", i), "status": 1, "remain_quota": 1000000, "key": "xx**yy"})
	}
	srv := httptest.NewServer(f.handler())
	defer srv.Close()
	got, err := newAPIAdapter{}.ListTokens(context.Background(), newSite(srv.URL))
	if err != nil || len(got) != 150 {
		t.Fatalf("len=%d err=%v", len(got), err)
	}
	if got[149].RemoteID != "150" || got[0].Key != "sk-full1" || got[0].QuotaUSD == nil || *got[0].QuotaUSD != 2 {
		t.Fatalf("unexpected %+v", got[0])
	}
}

func TestNewAPICreateToken(t *testing.T) {
	f := &fakeNewAPI{}
	srv := httptest.NewServer(f.handler())
	defer srv.Close()
	q := 3.0
	tok, err := newAPIAdapter{}.CreateToken(context.Background(), newSite(srv.URL), TokenCreate{Name: "nam-1", Group: "a", QuotaUSD: &q})
	if err != nil || tok.Name != "nam-1" || tok.Group != "a" || tok.Key != "sk-full1" || tok.QuotaUSD == nil || *tok.QuotaUSD != 3 {
		t.Fatalf("got %+v err=%v", tok, err)
	}
	un, err := newAPIAdapter{}.CreateToken(context.Background(), newSite(srv.URL), TokenCreate{Name: "nam-2"})
	if err != nil || !un.Unlimited || un.QuotaUSD != nil {
		t.Fatalf("got %+v err=%v", un, err)
	}
}

func TestNewAPIPricingMath(t *testing.T) {
	srv := httptest.NewServer((&fakeNewAPI{}).handler())
	defer srv.Close()
	p, err := newAPIAdapter{}.Pricing(context.Background(), newSite(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Groups) != 1 || p.Groups[0].Name != "a" || p.Groups[0].Ratio != 0.4 {
		t.Fatalf("groups %+v", p.Groups)
	}
	// m1 (group a only), m2 (empty enable_groups => all), m3 (zzz not usable => dropped)
	if len(p.Models) != 2 {
		t.Fatalf("models %+v", p.Models)
	}
	m1 := p.Models[0]
	if m1.Model != "m1" || !near(*m1.Input, 1.2) || !near(*m1.Output, 6.0) || !near(*m1.CacheRead, 0.12) || !near(*m1.ModelRatio, 1.5) {
		t.Fatalf("m1 %+v", m1)
	}
	if m2 := p.Models[1]; m2.PerRequest == nil || !near(*m2.PerRequest, 0.02) || m2.Input != nil {
		t.Fatalf("m2 %+v", m2)
	}
}

func near(a, b float64) bool { return a-b < 1e-9 && b-a < 1e-9 }

func TestNewAPIPasswordLoginFlow(t *testing.T) {
	f := &fakeNewAPI{}
	srv := httptest.NewServer(f.handler())
	defer srv.Close()
	s := Site{BaseURL: srv.URL, CredType: store.CredPassword, Cred: Credential{Username: "u", Password: "p"}}
	b, err := newAPIAdapter{}.Balance(context.Background(), s)
	if err != nil || b.USD != 5.0 {
		t.Fatalf("got %+v err=%v", b, err)
	}
	if f.logins != 1 || !f.cookieOK || f.lastUser != "42" || f.lastAuth != "" {
		t.Fatalf("logins=%d cookie=%v user=%q auth=%q", f.logins, f.cookieOK, f.lastUser, f.lastAuth)
	}
}

func TestNewAPIAnnouncements(t *testing.T) {
	srv := httptest.NewServer((&fakeNewAPI{}).handler())
	defer srv.Close()
	a, err := newAPIAdapter{}.Announcements(context.Background(), newSite(srv.URL))
	if err != nil || a.Content != "公告内容" {
		t.Fatalf("got %+v err=%v", a, err)
	}
}

func TestNewAPIListModelsSortedUnique(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"b"},{"id":"a"},{"id":"b"}]}`))
	}))
	defer srv.Close()
	got, err := newAPIAdapter{}.ListModels(context.Background(), Site{BaseURL: srv.URL}, "k1")
	if err != nil || fmt.Sprint(got) != "[a b]" {
		t.Fatalf("got %v err=%v", got, err)
	}
}

func TestRegistryTypes(t *testing.T) {
	if got := fmt.Sprint(Types()); got != "[generic newapi official onehub sk-key sub2api]" {
		t.Fatalf("got %s", got)
	}
}
