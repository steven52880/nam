package cachetest

import (
	"math"
	"strings"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestRateAnthropicUsesTotalInput(t *testing.T) {
	// Input is the normalized TOTAL (input + cache read + cache write).
	rounds := []Usage{
		{Round: 1, Input: 1010, CacheWrite: 1000},
		{Round: 2, Input: 1020, CacheRead: 1000},
		{Round: 3, Input: 1030, CacheRead: 1000},
	}
	r := Rate(rounds)
	if r == nil || !near(*r, 2000.0/2050.0) {
		t.Fatalf("rate = %v", r)
	}
	if got := SumRounds(rounds); got.Input != 2050 || got.CacheRead != 2000 || got.CacheWrite != 0 {
		t.Fatalf("sums = %+v", got)
	}
}

func TestRateTwoRoundsAndZeroDenominator(t *testing.T) {
	r := Rate([]Usage{{Input: 500}, {Input: 500, CacheRead: 250}})
	if r == nil || !near(*r, 0.5) {
		t.Fatalf("N=2 rate = %v", r)
	}
	if Rate([]Usage{{Input: 500}, {Input: 0}}) != nil {
		t.Fatal("zero denominator must give nil")
	}
	if Rate([]Usage{{Input: 500}}) != nil || Rate(nil) != nil {
		t.Fatal("fewer than 2 rounds must give nil")
	}
}

func TestParseAnthropicAddsCacheToInput(t *testing.T) {
	body := `{"content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":20,"output_tokens":7,
		"cache_read_input_tokens":1000,"cache_creation_input_tokens":5}}`
	u, ans, err := ParseResponse(ProtocolAnthropic, []byte(body))
	if err != nil || ans != "hi" {
		t.Fatalf("err=%v ans=%q", err, ans)
	}
	if u.Input != 1025 || u.CacheRead != 1000 || u.CacheWrite != 5 || u.Output != 7 {
		t.Fatalf("usage = %+v", u)
	}
}

func TestParseOpenAIKeepsPromptTokens(t *testing.T) {
	body := `{"choices":[{"message":{"content":"yo"}}],"usage":{"prompt_tokens":1000,"completion_tokens":3,
		"prompt_tokens_details":{"cached_tokens":900}}}`
	u, ans, err := ParseResponse(ProtocolOpenAI, []byte(body))
	if err != nil || ans != "yo" {
		t.Fatalf("err=%v ans=%q", err, ans)
	}
	if u.Input != 1000 || u.CacheRead != 900 || u.CacheWrite != 0 {
		t.Fatalf("usage = %+v", u)
	}
	if _, _, err := ParseResponse(ProtocolOpenAI, []byte(`{"choices":[]}`)); err != ErrNoUsage {
		t.Fatalf("want ErrNoUsage, got %v", err)
	}
}

func TestComposeKeyStripsSkPrefix(t *testing.T) {
	for _, tok := range []string{"abc", "sk-abc", "  sk-abc "} {
		if got := ComposeKey(tok, 12); got != "sk-abc-12" {
			t.Errorf("ComposeKey(%q) = %q", tok, got)
		}
	}
}

func TestCleanToken(t *testing.T) {
	if got, err := CleanToken(" sk-abcDEF123 "); err != nil || got != "abcDEF123" {
		t.Fatalf("got %q err %v", got, err)
	}
	for _, bad := range []string{"", "sk-", "a b", "abc-12"} {
		if _, err := CleanToken(bad); err == nil {
			t.Errorf("CleanToken(%q) should fail", bad)
		}
	}
	if h := TokenHint("abcdefghijklmnop"); !strings.HasPrefix(h, "sk-ab") || strings.Contains(h, "cdefghijklm") {
		t.Fatalf("hint leaks: %q", h)
	}
}

func TestBuildRoundShapes(t *testing.T) {
	a, err := BuildRound(ProtocolAnthropic, "claude-x", 2, []Turn{{Question: "q1", Answer: "a1"}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(a)
	if !strings.Contains(s, `"cache_control":{"type":"ephemeral"}`) || !strings.Contains(s, `"system":[`) {
		t.Fatalf("anthropic body lacks cache_control system block")
	}
	o, _ := BuildRound(ProtocolOpenAI, "gpt-x", 1, nil)
	if strings.Contains(string(o), "cache_control") || !strings.Contains(string(o), `"role":"system"`) {
		t.Fatalf("openai body wrong")
	}
	if len(corpus) < 40000 {
		t.Fatalf("corpus too short: %d", len(corpus))
	}
}
