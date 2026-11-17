package stats

import (
	"math"
	"strconv"
	"testing"

	"github.com/steven52880/nam/ports"
)

func consume(other string) ports.LogRecord {
	return ports.LogRecord{
		ID: 1, CreatedAt: 7300, Type: ports.LogTypeConsume, ModelName: "m", ChannelID: 3, Group: "g",
		PromptTokens: 1000, CompletionTokens: 500, IsStream: true, UseTime: 3, Quota: 1743, Other: other,
	}
}

func mustParse(t *testing.T, r ports.LogRecord) Sample {
	t.Helper()
	s, ok := Parse(r)
	if !ok {
		t.Fatalf("Parse(%+v) skipped", r)
	}
	return s
}

func near(a, b, eps float64) bool { return math.Abs(a-b) <= eps }

func TestParse_openai_semantic_subtracts_cache_from_prompt(t *testing.T) {
	s := mustParse(t, consume(`{"cache_tokens":200,"cache_creation_tokens":100}`))
	if s.Input != 700 || s.Read != 200 || s.W5 != 100 || s.W1 != 0 || s.Output != 500 {
		t.Fatalf("tokens %+v", s)
	}
	if !s.OK || s.Hour != 7200 || s.Quota != 1743 || s.ChannelID != 3 || s.Group != "g" {
		t.Fatalf("meta %+v", s)
	}
}

func TestParse_anthropic_semantic_keeps_prompt_as_input(t *testing.T) {
	s := mustParse(t, consume(`{"cache_tokens":200,"usage_semantic":"anthropic"}`))
	if s.Input != 1000 || s.Read != 200 {
		t.Fatalf("tokens %+v", s)
	}
}

func TestParse_cache_creation_split_rest_goes_to_5m(t *testing.T) {
	s := mustParse(t, consume(`{"cache_creation_tokens":300,"cache_creation_tokens_5m":50,"cache_creation_tokens_1h":100}`))
	if s.W5 != 200 || s.W1 != 100 || s.Input != 700 {
		t.Fatalf("split %+v", s)
	}
}

func TestParse_cache_creation_less_than_parts_never_negative(t *testing.T) {
	s := mustParse(t, consume(`{"cache_creation_tokens":10,"cache_creation_tokens_5m":50,"cache_creation_tokens_1h":100}`))
	if s.W5 != 50 || s.W1 != 100 {
		t.Fatalf("split %+v", s)
	}
}

func TestParse_frt_only_for_stream_and_positive(t *testing.T) {
	cases := []struct {
		name   string
		stream bool
		other  string
		want   float64
	}{
		{"stream positive", true, `{"frt":300}`, 300},
		{"non-stream -1000", false, `{"frt":-1000}`, 0},
		{"stream zero", true, `{"frt":0}`, 0},
		{"non-stream positive", false, `{"frt":300}`, 0},
		{"missing other", true, ``, 0},
	}
	for _, c := range cases {
		r := consume(c.other)
		r.IsStream = c.stream
		if s := mustParse(t, r); s.FRT != c.want {
			t.Errorf("%s: frt %v want %v", c.name, s.FRT, c.want)
		}
	}
}

func TestParse_speed_exact_with_use_time_ms(t *testing.T) {
	s := mustParse(t, consume(`{"frt":300,"use_time_ms":2300}`))
	if !near(s.Speed, 250, 1e-9) || s.SpeedEst || !s.UseTimeMS {
		t.Fatalf("speed %+v", s)
	}
}

func TestParse_speed_estimated_from_whole_seconds(t *testing.T) {
	r := consume(`{"frt":300}`)
	r.UseTime = 2
	s := mustParse(t, r)
	if !near(s.Speed, 500/1.7, 1e-9) || !s.SpeedEst || s.UseTimeMS {
		t.Fatalf("speed %+v", s)
	}
}

func TestParse_speed_dropped_when_denominator_not_positive(t *testing.T) {
	r := consume(`{"frt":3000}`)
	r.UseTime = 2
	if s := mustParse(t, r); s.Speed != 0 || s.FRT != 3000 {
		t.Fatalf("speed %+v", s)
	}
}

func TestParse_speed_dropped_without_completion(t *testing.T) {
	r := consume(`{"frt":300,"use_time_ms":2300}`)
	r.CompletionTokens = 0
	if s := mustParse(t, r); s.Speed != 0 {
		t.Fatalf("speed %+v", s)
	}
}

func TestParse_error_classes(t *testing.T) {
	cases := []struct {
		status  int
		content string
		want    string
	}{
		{504, "", ClassTimeout},
		{408, "", ClassTimeout},
		{0, "context Deadline Exceeded", ClassTimeout},
		{500, "请求超时", ClassTimeout},
		{429, "", Class429},
		{401, "", Class401},
		{403, "", Class401},
		{402, "", ClassBalance},
		{400, "Insufficient_Quota", ClassBalance},
		{0, "余额不足", ClassBalance},
		{0, "额度不足", ClassBalance},
		{0, "insufficient balance", ClassBalance},
		{400, "no credit left", ClassBalance},
		{500, "", Class5xx},
		{599, "", Class5xx},
		{400, "bad request", ClassOther},
		{0, "", ClassOther},
	}
	for _, c := range cases {
		r := ports.LogRecord{ID: 1, Type: ports.LogTypeError, ChannelID: 1, Content: c.content,
			Other: `{"status_code":` + strconv.Itoa(c.status) + `}`}
		s := mustParse(t, r)
		if s.OK || s.FailClass != c.want {
			t.Errorf("status %d %q: class %q want %q", c.status, c.content, s.FailClass, c.want)
		}
	}
}

func TestParse_skips_tests_other_types_and_unknown_channel(t *testing.T) {
	test := consume("")
	test.TokenName = TestTokenName
	topup := consume("")
	topup.Type = 1
	noCh := consume("")
	noCh.ChannelID = 0
	for _, r := range []ports.LogRecord{test, topup, noCh} {
		if _, ok := Parse(r); ok {
			t.Errorf("expected skip: %+v", r)
		}
	}
}
