package core_test

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/steven52880/nam/ports"
)

// Plan §11.1 M2: upstream → sync → vendor → batch → apply → batch edit → drift → delete.
func TestM2EndToEnd(t *testing.T) {
	m := newM2(t)
	ctx := context.Background()

	var up struct {
		ID uint `json:"id"`
	}
	m.must("POST", "/upstreams", map[string]any{"name": "mock-a", "type": "newapi", "base_url": m.site.URL, "cred_type": "access_token",
		"credential": map[string]any{"access_token": "tok"}, "platform_user_id": 42, "currency_rate": 0.5}).into(t, &up)
	base := "/upstreams/" + strconv.Itoa(int(up.ID))

	var sync []struct {
		What  string `json:"what"`
		OK    bool   `json:"ok"`
		Count int    `json:"count"`
	}
	m.must("POST", base+"/sync", map[string]any{}).into(t, &sync)
	if len(sync) != 3 || !sync[0].OK || sync[0].Count != 3 || !sync[1].OK || sync[1].Count != 2 || !sync[2].OK {
		t.Fatalf("sync: %+v", sync)
	}
	var tokens []struct {
		ID      uint   `json:"id"`
		Name    string `json:"name"`
		KeyHint string `json:"key_hint"`
	}
	tr := m.must("GET", base+"/tokens", nil)
	tr.into(t, &tokens)
	if len(tokens) != 3 || tokens[0].KeyHint != "sk-ab…kl-1" {
		t.Fatalf("tokens: %+v", tokens)
	}
	if strings.Contains(tr.Raw, "abcdefghijkl") || strings.Contains(tr.Raw, `"key":`) {
		t.Fatalf("token key leaked: %s", tr.Raw)
	}
	if r := m.must("GET", base+"/groups", nil); !strings.Contains(r.Raw, `"ratio":0.4`) {
		t.Fatalf("groups: %s", r.Raw)
	}
	if r := m.must("GET", base+"/models", nil); !strings.Contains(r.Raw, "test-model") {
		t.Fatalf("models: %s", r.Raw)
	}

	// vendor + overlap rejection
	m.must("POST", "/vendors", map[string]any{"name": "anthropic", "group_name": "anthropic", "models": []string{"test-model"}})
	if r := m.call("POST", "/vendors", map[string]any{"name": "other", "group_name": "other", "models": []string{"test-model"}}); r.Code != 409 ||
		!strings.Contains(r.Body.Message, "anthropic") || !strings.Contains(r.Body.Message, "other") {
		t.Fatalf("overlap: %d %s", r.Code, r.Raw)
	}
	if r := m.call("POST", "/vendors", map[string]any{"name": "bad", "group_name": "has space", "models": []string{}}); r.Code != 400 {
		t.Fatalf("group chars: %d", r.Code)
	}
	if r := m.call("POST", "/vendors", map[string]any{"name": "bad", "group_name": "bad", "channel_type": 1}); r.Code != 400 {
		t.Fatalf("channel type: %d", r.Code)
	}

	// batch preview + create + apply
	ids := make([]uint, 0, 3)
	for _, tk := range tokens {
		ids = append(ids, tk.ID)
	}
	tokenModels := map[string][]string{}
	for _, id := range ids {
		tokenModels[strconv.Itoa(int(id))] = []string{"test-model", "mystery"}
	}
	req := map[string]any{"upstream_id": up.ID, "token_ids": ids, "token_models": tokenModels, "weight": 1}
	var prev struct {
		Items      []struct{ Name string } `json:"items"`
		Unassigned []string                `json:"unassigned"`
	}
	m.must("POST", "/channels/batch/preview", req).into(t, &prev)
	if len(prev.Items) != 3 || len(prev.Unassigned) != 1 || prev.Unassigned[0] != "mystery" {
		t.Fatalf("preview: %+v", prev)
	}
	m.must("POST", "/channels/batch", req)
	var plan []struct {
		Op string `json:"op"`
	}
	m.must("GET", "/channels/plan", nil).into(t, &plan)
	if len(plan) != 3 || plan[0].Op != "create" {
		t.Fatalf("plan: %+v", plan)
	}
	var ap struct {
		Success int `json:"success"`
		Failed  int `json:"failed"`
	}
	m.must("POST", "/channels/apply", map[string]any{}).into(t, &ap)
	if ap.Success != 3 || ap.Failed != 0 {
		t.Fatalf("apply: %+v", ap)
	}
	live, _ := m.fake.List(ctx)
	if len(live) != 3 {
		t.Fatalf("fake channels: %d", len(live))
	}
	wantKey := map[string]string{"nam-mock-a-a-anthropic": "sk-abcdefghijkl-1", "nam-mock-a-b-anthropic": "sk-abcdefghijkl-2", "nam-mock-a-c-anthropic": "sk-abcdefghijkl-3"}
	for _, c := range live {
		if c.Type != 58 || c.Group != "anthropic" || c.Key != wantKey[c.Name] || len(c.Models) != 1 || c.Models[0] != "test-model" {
			t.Fatalf("channel %s: %+v want key %q", c.Name, c, wantKey[c.Name])
		}
	}

	// batch priority change → apply → 3 new audit rows
	before := m.auditTotal("channel")
	var changes []map[string]any
	for _, c := range live {
		changes = append(changes, map[string]any{"channel_id": c.ID, "fields": map[string]any{"priority": 5}})
	}
	m.must("PATCH", "/channels/drafts", map[string]any{"changes": changes})
	m.must("POST", "/channels/apply", map[string]any{})
	live, _ = m.fake.List(ctx)
	for _, c := range live {
		if c.Priority != 5 {
			t.Fatalf("priority: %+v", c)
		}
	}
	if got := m.auditTotal("channel") - before; got != 3 {
		t.Fatalf("new channel audit rows: %d", got)
	}
	// validation failure → 400 with issues
	r := m.call("PATCH", "/channels/drafts", map[string]any{"changes": []map[string]any{{"channel_id": live[0].ID, "fields": map[string]any{"group": "nope"}}}})
	if r.Code != 400 || !strings.Contains(r.Raw, `"issues"`) {
		t.Fatalf("validation: %d %s", r.Code, r.Raw)
	}

	// drift: direct NewAPI edit, consistency job flags it, nothing is written back
	w := uint(9)
	if err := m.fake.Update(ctx, live[0].ID, ports.ChannelPatch{Weight: &w}); err != nil {
		t.Fatal(err)
	}
	writes := m.fake.Writes
	m.must("POST", "/jobs/consistency?wait=1", nil)
	var drifted bool
	for _, row := range m.channels() {
		if row.ID == live[0].ID && row.SyncState == "drift" && len(row.DriftFields) == 1 && row.DriftFields[0] == "weight" {
			drifted = true
		}
	}
	if !drifted || m.fake.Writes != writes {
		t.Fatalf("drift=%v writes %d→%d", drifted, writes, m.fake.Writes)
	}
	if c, _ := m.fake.Get(ctx, live[0].ID); c.Weight != 9 {
		t.Fatalf("drift check must not change NewAPI: %d", c.Weight)
	}
	if r := m.must("GET", "/channels/"+strconv.Itoa(live[0].ID)+"/newapi-link", nil); !strings.Contains(r.Raw, "/console/channel?id=") {
		t.Fatalf("link: %s", r.Raw)
	}

	// token in use cannot be deleted; deleting the upstream drafts deletes for its channels
	if r := m.call("DELETE", base+"/tokens/"+strconv.Itoa(int(tokens[0].ID)), nil); r.Code != 409 {
		t.Fatalf("token in use: %d %s", r.Code, r.Raw)
	}
	if r := m.call("DELETE", "/vendors/1", nil); r.Code != 409 {
		t.Fatalf("vendor in use: %d %s", r.Code, r.Raw)
	}
	var del struct {
		DeletedChannels int `json:"deleted_channels"`
	}
	m.must("DELETE", base, nil).into(t, &del)
	if del.DeletedChannels != 3 {
		t.Fatalf("deleted_channels: %d", del.DeletedChannels)
	}
	for _, row := range m.channels() {
		if row.SyncState != "draft_delete" {
			t.Fatalf("expected draft_delete: %+v", row)
		}
	}
}
