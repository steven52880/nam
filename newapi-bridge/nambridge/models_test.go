package nambridge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestModelFetcherOpenAIStyle(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"a"},{"id":"b"}]}`))
	}))
	t.Cleanup(srv.Close)

	got, err := modelFetcher{}.FetchModels(context.Background(), srv.URL, 58, "  sk-test\nsecond  ")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("models = %v", got)
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("NewAPI must trim the key to its first line, got Authorization %q", gotAuth)
	}
}

func TestModelFetcherUpstreamFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	_, err := modelFetcher{}.FetchModels(context.Background(), srv.URL, 58, "sk-secret-key")
	if err == nil {
		t.Fatal("non-200 upstream must be an error")
	}
	if strings.Contains(err.Error(), "sk-secret-key") {
		t.Fatalf("error leaks the key: %v", err)
	}
}

func TestModelFetcherInPorts(t *testing.T) {
	if newPorts().Models == nil {
		t.Fatal("embedded ports must carry a ModelFetcher")
	}
}
