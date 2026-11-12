package core_test

import (
	"strconv"
	"testing"
)

type m35Token struct {
	ID     uint     `json:"id"`
	Name   string   `json:"name"`
	Models []string `json:"models"`
}

func TestM35TokenModelsAndMatchTokens(t *testing.T) {
	m := newM2(t)
	upID, chans := m.setupChannels()
	base := "/upstreams/" + strconv.Itoa(int(upID))

	var toks []m35Token
	m.must("GET", base+"/tokens", nil).into(t, &toks)
	if len(toks) != 3 || toks[0].Models == nil || len(toks[0].Models) != 0 {
		t.Fatalf("tokens must list models as []: %+v", toks)
	}

	tb := base + "/tokens/" + strconv.Itoa(int(toks[0].ID)) + "/models"
	var saved m35Token
	m.must("PUT", tb, map[string]any{"models": []string{" m1 ", "", "m2", "m1"}}).into(t, &saved)
	if len(saved.Models) != 2 || saved.Models[0] != "m1" || saved.Models[1] != "m2" {
		t.Fatalf("saved: %+v", saved)
	}
	m.must("GET", base+"/tokens", nil).into(t, &toks)
	if len(toks[0].Models) != 2 {
		t.Fatalf("models not persisted: %+v", toks[0])
	}
	// sync must not wipe them
	m.must("POST", base+"/sync", map[string]any{})
	m.must("GET", base+"/tokens", nil).into(t, &toks)
	if len(toks[0].Models) != 2 {
		t.Fatalf("models lost by sync: %+v", toks[0])
	}
	if r := m.call("PUT", "/upstreams/9999/tokens/"+strconv.Itoa(int(toks[0].ID))+"/models", map[string]any{"models": []string{"x"}}); r.Code != 404 {
		t.Fatalf("wrong upstream: %d", r.Code)
	}

	// match-tokens: applied channel for token a carries token a's key → matched, nothing created
	var match struct {
		Items []struct {
			ChannelID   int   `json:"channel_id"`
			TokenID     *uint `json:"token_id"`
			Created     bool  `json:"created"`
			NeedsManual bool  `json:"needs_manual"`
		} `json:"items"`
	}
	m.must("POST", "/channels/match-tokens", map[string]any{"upstream_id": upID, "channel_ids": []int{chans["a"]}}).into(t, &match)
	if len(match.Items) != 1 || match.Items[0].Created || match.Items[0].NeedsManual || match.Items[0].TokenID == nil || *match.Items[0].TokenID != toks[0].ID {
		t.Fatalf("match: %+v", match)
	}
	if r := m.call("POST", "/channels/match-tokens", map[string]any{"upstream_id": upID, "channel_ids": []int{}}); r.Code != 400 {
		t.Fatalf("empty channel_ids: %d", r.Code)
	}
}
