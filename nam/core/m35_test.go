package core_test

import (
	"strconv"
	"testing"
)

type m35Token struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

func TestM35MatchTokens(t *testing.T) {
	m := newM2(t)
	upID, chans := m.setupChannels()
	base := "/upstreams/" + strconv.Itoa(int(upID))

	var toks []m35Token
	m.must("GET", base+"/tokens", nil).into(t, &toks)
	if len(toks) != 3 {
		t.Fatalf("tokens: %+v", toks)
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
