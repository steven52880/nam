package standalone

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestModelFetcher(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/channel/fetch_models" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != fakeToken || r.Header.Get("New-Api-User") != strconv.Itoa(fakeUserID) {
			w.WriteHeader(http.StatusUnauthorized)
			reply(w, false, "未登录", nil)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		if gotBody["key"] == "bad" {
			reply(w, false, "Failed to fetch models", nil)
			return
		}
		reply(w, true, "", []string{"a", "b"})
	}))
	t.Cleanup(srv.Close)
	cfg := Config{NewAPIURL: srv.URL, Token: fakeToken, UserID: fakeUserID, Secret: "x", AdminPassword: "initial-pass"}
	p, _, err := build(cfg, newClient(cfg, time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if p.Models == nil {
		t.Fatal("standalone ports must carry a ModelFetcher")
	}

	got, err := p.Models.FetchModels(context.Background(), "https://up.example", 58, "sk-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("models = %v", got)
	}
	if gotBody["base_url"] != "https://up.example" || gotBody["type"] != float64(58) || gotBody["key"] != "sk-1" {
		t.Fatalf("request body = %v", gotBody)
	}

	_, err = p.Models.FetchModels(context.Background(), "https://up.example", 58, "bad")
	if err == nil || !strings.Contains(err.Error(), "Failed to fetch models") {
		t.Fatalf("success=false must surface the message, got %v", err)
	}
	if strings.Contains(err.Error(), "bad") && strings.Contains(err.Error(), "key") {
		t.Fatalf("error must not carry the key: %v", err)
	}
}
