package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/store"
)

func TestUpstreamViewIncludesSyncParts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := newCredTestServer(t)
	ctx := context.Background()
	u := store.Upstream{Name: "u", Type: store.UpstreamNewAPI, BaseURL: "https://a.example", CurrencyRate: 1}
	if err := s.Store.SaveUpstream(ctx, &u); err != nil {
		t.Fatal(err)
	}
	call := func(h gin.HandlerFunc, params gin.Params) map[string]any {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c.Params = params
		h(c)
		var body struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: %v", w.Body.String(), err)
		}
		var one map[string]any
		if err := json.Unmarshal(body.Data, &one); err != nil {
			var many []map[string]any
			if err := json.Unmarshal(body.Data, &many); err != nil || len(many) != 1 {
				t.Fatalf("body: %s", w.Body.String())
			}
			one = many[0]
		}
		return one
	}
	params := gin.Params{{Key: "id", Value: "1"}}

	// never synced: an empty object, not null
	for name, got := range map[string]map[string]any{"list": call(s.listUpstreams, nil), "get": call(s.getUpstream, params)} {
		if m, ok := got["sync_parts"].(map[string]any); !ok || len(m) != 0 {
			t.Fatalf("%s before sync: %v", name, got["sync_parts"])
		}
	}

	err := s.Store.DB.Exec(`UPDATE upstreams SET sync_parts = json_set('{}', '$.groups', json(?))`,
		`{"state":"ok","at":1794990257,"message":"","count":13}`).Error
	if err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]map[string]any{"list": call(s.listUpstreams, nil), "get": call(s.getUpstream, params)} {
		m, _ := got["sync_parts"].(map[string]any)
		g, _ := m["groups"].(map[string]any)
		if g["state"] != "ok" || g["count"] != float64(13) || g["at"] != float64(1794990257) {
			t.Fatalf("%s: %v", name, got["sync_parts"])
		}
	}

	// saving the upstream (edit form) must not clobber sync_parts
	got, _ := s.Store.GetUpstream(ctx, u.ID)
	got.Remark = "changed"
	if err := s.Store.SaveUpstream(ctx, &got); err != nil {
		t.Fatal(err)
	}
	again, _ := s.Store.GetUpstream(ctx, u.ID)
	if again.Remark != "changed" || again.SyncParts["groups"].State != "ok" {
		t.Fatalf("SaveUpstream clobbered sync_parts: %+v", again.SyncParts)
	}
}
