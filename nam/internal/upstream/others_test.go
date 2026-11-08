package upstream

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSKKeyBalanceFollowsBillingSemantics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/v1/dashboard/billing/subscription":
			_, _ = w.Write([]byte(`{"object":"billing_subscription","hard_limit_usd":10}`))
		case "/v1/dashboard/billing/usage":
			// used 2.5 USD => total_usage in cents = 250
			_, _ = w.Write([]byte(`{"object":"list","total_usage":250}`))
		}
	}))
	defer srv.Close()
	s := Site{BaseURL: srv.URL, Cred: Credential{Key: "sk-1"}}
	b, err := skKeyAdapter{}.Balance(context.Background(), s)
	if err != nil || b.USD != 7.5 || b.Unlimited {
		t.Fatalf("got %+v err=%v", b, err)
	}
	if err := (skKeyAdapter{}).Verify(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	s.Cred.Key = "bad"
	if err := (skKeyAdapter{}).Verify(context.Background(), s); !errors.Is(err, ErrAuth) {
		t.Fatalf("want ErrAuth, got %v", err)
	}
}

func TestSKKeyUnlimitedAndTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"hard_limit_usd":100000000}`))
	}))
	defer srv.Close()
	s := Site{BaseURL: srv.URL, Cred: Credential{Key: "sk-1"}}
	b, err := skKeyAdapter{}.Balance(context.Background(), s)
	if err != nil || !b.Unlimited {
		t.Fatalf("got %+v err=%v", b, err)
	}
	toks, err := skKeyAdapter{}.ListTokens(context.Background(), s)
	if err != nil || len(toks) != 1 || toks[0].Key != "sk-1" || toks[0].Name != "(sk-key)" || toks[0].Status != 1 {
		t.Fatalf("got %+v err=%v", toks, err)
	}
	if _, err := (skKeyAdapter{}).CreateToken(context.Background(), s, TokenCreate{}); !errors.Is(err, ErrUnsupported) {
		t.Fatal("create should be unsupported")
	}
	if _, err := (skKeyAdapter{}).Pricing(context.Background(), s); !errors.Is(err, ErrUnsupported) {
		t.Fatal("pricing should be unsupported")
	}
}

func TestOfficialListModelsPerProvider(t *testing.T) {
	var gotPath, gotAuth, gotXKey, gotVer, gotQueryKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		gotXKey, gotVer, gotQueryKey = r.Header.Get("x-api-key"), r.Header.Get("anthropic-version"), r.URL.Query().Get("key")
		if strings.HasPrefix(r.URL.Path, "/v1beta/") {
			_, _ = w.Write([]byte(`{"models":[{"name":"models/gemini-b"},{"name":"models/gemini-a"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"m2"},{"id":"m1"}]}`))
	}))
	defer srv.Close()
	ctx := context.Background()

	openai, err := officialAdapter{}.ListModels(ctx, Site{BaseURL: srv.URL}, "k")
	if err != nil || strings.Join(openai, ",") != "m1,m2" || gotAuth != "Bearer k" || gotXKey != "" {
		t.Fatalf("openai got=%v err=%v auth=%q", openai, err, gotAuth)
	}
	anth, err := listModelsAnthropic(ctx, Site{BaseURL: srv.URL}, "ak")
	if err != nil || strings.Join(anth, ",") != "m1,m2" || gotXKey != "ak" || gotVer != "2023-06-01" || gotAuth != "" || gotPath != "/v1/models" {
		t.Fatalf("anthropic got=%v err=%v x=%q ver=%q auth=%q", anth, err, gotXKey, gotVer, gotAuth)
	}
	g, err := listModelsGoogle(ctx, Site{BaseURL: srv.URL}, "gk")
	if err != nil || strings.Join(g, ",") != "gemini-a,gemini-b" || gotQueryKey != "gk" || gotAuth != "" {
		t.Fatalf("google got=%v err=%v key=%q", g, err, gotQueryKey)
	}
}

func TestOfficialDispatchByBaseURL(t *testing.T) {
	var seen string
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seen = r.URL.Host + "|" + r.Header.Get("x-api-key") + "|" + r.URL.Query().Get("key")
		return &http.Response{StatusCode: 200, Body: http.NoBody, Header: http.Header{}, Request: r}, nil
	})
	hc := &http.Client{Transport: rt}
	ctx := context.Background()
	_, _ = officialAdapter{}.ListModels(ctx, Site{BaseURL: "https://api.anthropic.com", HTTP: hc}, "ak")
	if seen != "api.anthropic.com|ak|" {
		t.Fatalf("anthropic dispatch: %s", seen)
	}
	_, _ = officialAdapter{}.ListModels(ctx, Site{BaseURL: "https://generativelanguage.googleapis.com", HTTP: hc}, "gk")
	if seen != "generativelanguage.googleapis.com||gk" {
		t.Fatalf("google dispatch: %s", seen)
	}
	if r, _ := (officialAdapter{}).Probe(ctx, Site{}); r.Evidence != "官方直连，不检测" || r.Type != "official" {
		t.Fatalf("probe %+v", r)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGenericBalanceAndPlaceholders(t *testing.T) {
	var gotHdr, gotTok string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHdr, gotTok = r.Header.Get("X-Key"), r.Header.Get("X-Tok")
		switch r.URL.Path {
		case "/_custom/bal":
			_, _ = w.Write([]byte(`{"data":{"left":12345,"items":[{"x":7}]}}`))
		case "/notice":
			_, _ = w.Write([]byte("  hello \n"))
		}
	}))
	defer srv.Close()
	cfg := `{"balance":{"url":"/_custom/bal","headers":{"X-Key":"Bearer {{key}}","X-Tok":"{{token}}"},"json_path":"data.left","factor":0.01},"announcement_url":"/notice"}`
	s := Site{BaseURL: srv.URL, Config: cfg, Cred: Credential{Key: "K", AccessToken: "T"}}
	b, err := genericAdapter{}.Balance(context.Background(), s)
	if err != nil || !near(b.USD, 123.45) {
		t.Fatalf("got %+v err=%v", b, err)
	}
	if gotHdr != "Bearer K" || gotTok != "T" {
		t.Fatalf("headers %q %q", gotHdr, gotTok)
	}
	s.Config = strings.Replace(cfg, "data.left", "data.items.0.x", 1)
	if b, err := (genericAdapter{}).Balance(context.Background(), s); err != nil || !near(b.USD, 0.07) {
		t.Fatalf("array path got %+v err=%v", b, err)
	}
	a, err := genericAdapter{}.Announcements(context.Background(), s)
	if err != nil || a.Content != "hello" {
		t.Fatalf("got %+v err=%v", a, err)
	}
	if _, err := (genericAdapter{}).Announcements(context.Background(), Site{BaseURL: srv.URL}); !errors.Is(err, ErrUnsupported) {
		t.Fatal("want ErrUnsupported without announcement_url")
	}
	if _, err := (genericAdapter{}).ListTokens(context.Background(), Site{BaseURL: srv.URL}); !errors.Is(err, ErrUnsupported) {
		t.Fatal("want ErrUnsupported without key")
	}
	if r, err := (genericAdapter{}).Probe(context.Background(), Site{BaseURL: srv.URL}); err != nil || r.Evidence != "可访问（HTTP 200）" {
		t.Fatalf("probe %+v err=%v", r, err)
	}
}

func TestValidateGenericConfig(t *testing.T) {
	cases := map[string]bool{
		``:   true,
		`{}`: true,
		`{"balance":{"url":"/b","json_path":"a.b","factor":0.01}}`:   true,
		`{"balance":{"json_path":"a"}}`:                              false,
		`{"balance":{"url":"/b"}}`:                                   false,
		`{"balance":{"url":"/b","json_path":"a","method":"DELETE"}}`: false,
		`{not json`: false,
	}
	for raw, ok := range cases {
		if err := ValidateGenericConfig(raw); (err == nil) != ok {
			t.Errorf("%q: ok=%v err=%v", raw, ok, err)
		}
	}
}
