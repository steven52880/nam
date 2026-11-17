package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestServer returns a server with fast latency for tests.
func newTestServer() *server {
	s := newServer()
	s.latencyScale = 0.01
	return s
}

func do(s *server, method, target string, body string, headers map[string]string) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

func TestCallsRecordInjectedStatus(t *testing.T) {
	s := newTestServer()
	do(s, http.MethodPost, "/_ctl", `{"fail_next":1,"fail_status":429}`, nil)
	do(s, http.MethodPost, "/v1/chat/completions", `{"model":"m"}`, nil)
	do(s, http.MethodPost, "/v1/chat/completions", `{"model":"m"}`, nil)
	calls := s.snapshotCalls()
	if len(calls) != 2 || calls[0].Status != 429 || calls[1].Status != 200 {
		t.Fatalf("calls = %+v, want statuses 429 then 200", calls)
	}
}

func TestChatNonStreamUsage(t *testing.T) {
	s := newTestServer()
	rec := do(s, http.MethodPost, "/v1/chat/completions", `{"model":"test-model","messages":[]}`, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens        int `json:"prompt_tokens"`
			CompletionTokens    int `json:"completion_tokens"`
			TotalTokens         int `json:"total_tokens"`
			PromptTokensDetails struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Usage.PromptTokens != 1000 || resp.Usage.CompletionTokens != 500 || resp.Usage.TotalTokens != 1500 {
		t.Fatalf("usage = %+v, want 1000/500/1500", resp.Usage)
	}
	if resp.Usage.PromptTokensDetails.CachedTokens != 200 {
		t.Fatalf("cached_tokens = %d, want 200", resp.Usage.PromptTokensDetails.CachedTokens)
	}
	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		t.Fatalf("missing message content")
	}
}

func TestChatStreamEndsWithDoneAndUsage(t *testing.T) {
	s := newTestServer()
	rec := do(s, http.MethodPost, "/v1/chat/completions", `{"model":"test-model","stream":true}`, nil)

	body := rec.Body.String()
	if !strings.Contains(body, "data: [DONE]") {
		t.Fatalf("stream missing [DONE] terminator")
	}
	if !strings.Contains(body, "chat.completion.chunk") {
		t.Fatalf("stream missing chunk objects")
	}
	if !strings.Contains(body, `"prompt_tokens":1000`) {
		t.Fatalf("stream missing usage numbers")
	}
	if !strings.Contains(body, `"cached_tokens":200`) {
		t.Fatalf("stream missing cached tokens")
	}
}

func TestMessagesStreamHasStartAndStop(t *testing.T) {
	s := newTestServer()
	rec := do(s, http.MethodPost, "/v1/messages", `{"model":"test-model","stream":true}`, nil)

	body := rec.Body.String()
	if !strings.Contains(body, "event: message_start") {
		t.Fatalf("stream missing message_start")
	}
	if !strings.Contains(body, "event: message_stop") {
		t.Fatalf("stream missing message_stop")
	}
}

func TestGenerateContentNonStream(t *testing.T) {
	s := newTestServer()
	rec := do(s, http.MethodPost, "/v1beta/models/gemini-pro:generateContent", `{}`, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount        int `json:"promptTokenCount"`
			CachedContentTokenCount int `json:"cachedContentTokenCount"`
			CandidatesTokenCount    int `json:"candidatesTokenCount"`
			TotalTokenCount         int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 || resp.Candidates[0].Content.Parts[0].Text == "" {
		t.Fatalf("missing candidate text")
	}
	if resp.UsageMetadata.PromptTokenCount != 1000 || resp.UsageMetadata.CachedContentTokenCount != 200 ||
		resp.UsageMetadata.CandidatesTokenCount != 500 || resp.UsageMetadata.TotalTokenCount != 1500 {
		t.Fatalf("usageMetadata = %+v, want 1000/200/500/1500", resp.UsageMetadata)
	}
}

func TestGenerateContentStream(t *testing.T) {
	s := newTestServer()
	rec := do(s, http.MethodPost, "/v1beta/models/gemini-pro:streamGenerateContent?alt=sse", `{}`, nil)

	body := rec.Body.String()
	if !strings.Contains(body, "data: ") {
		t.Fatalf("stream missing SSE data lines")
	}
	if !strings.Contains(body, `"usageMetadata"`) {
		t.Fatalf("stream missing usageMetadata")
	}
}

func TestFailInjection(t *testing.T) {
	s := newTestServer()
	rec := do(s, http.MethodPost, "/_ctl", `{"fail_next":2,"fail_status":500}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("ctl status = %d, want 200", rec.Code)
	}

	for i := 0; i < 2; i++ {
		r := do(s, http.MethodPost, "/v1/chat/completions", `{"model":"test-model"}`, nil)
		if r.Code != http.StatusInternalServerError {
			t.Fatalf("call %d status = %d, want 500", i+1, r.Code)
		}
		var errBody struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(r.Body.Bytes(), &errBody); err != nil {
			t.Fatalf("decode error body: %v", err)
		}
		if errBody.Error.Message != "mock injected error" {
			t.Fatalf("error message = %q", errBody.Error.Message)
		}
	}

	third := do(s, http.MethodPost, "/v1/chat/completions", `{"model":"test-model"}`, nil)
	if third.Code != http.StatusOK {
		t.Fatalf("third call status = %d, want 200", third.Code)
	}
}

func TestCallsRecordsAuthStyles(t *testing.T) {
	s := newTestServer()
	do(s, http.MethodPost, "/v1/chat/completions", `{"model":"test-model"}`,
		map[string]string{"Authorization": "Bearer sk-1"})
	do(s, http.MethodPost, "/v1/messages", `{"model":"test-model"}`,
		map[string]string{"x-api-key": "sk-2"})
	do(s, http.MethodPost, "/v1beta/models/gemini-pro:generateContent", `{}`,
		map[string]string{"x-goog-api-key": "sk-3"})

	rec := do(s, http.MethodGet, "/_calls", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("calls status = %d, want 200", rec.Code)
	}
	var calls []callRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &calls); err != nil {
		t.Fatalf("decode calls: %v", err)
	}
	if len(calls) != 3 {
		t.Fatalf("recorded %d calls, want 3", len(calls))
	}
	if calls[0].AuthHeader != "Bearer sk-1" {
		t.Fatalf("auth_header = %q, want Bearer sk-1", calls[0].AuthHeader)
	}
	if calls[1].XAPIKey != "sk-2" {
		t.Fatalf("x_api_key = %q, want sk-2", calls[1].XAPIKey)
	}
	if calls[2].XGoogAPIKey != "sk-3" {
		t.Fatalf("x_goog_api_key = %q, want sk-3", calls[2].XGoogAPIKey)
	}
	if calls[0].Path == "" || calls[0].Model != "test-model" {
		t.Fatalf("path/model not recorded: %+v", calls[0])
	}
}

func TestResetClearsCallsAndState(t *testing.T) {
	s := newTestServer()
	do(s, http.MethodPost, "/v1/chat/completions", `{"model":"test-model"}`, nil)
	rec := do(s, http.MethodPost, "/_ctl", `{"reset":true}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("reset status = %d, want 200", rec.Code)
	}
	callsRec := do(s, http.MethodGet, "/_calls", "", nil)
	var calls []callRecord
	if err := json.Unmarshal(callsRec.Body.Bytes(), &calls); err != nil {
		t.Fatalf("decode calls: %v", err)
	}
	if len(calls) != 0 {
		t.Fatalf("calls after reset = %d, want 0", len(calls))
	}
}

func TestFailKeyOnlyFailsMatchingKey(t *testing.T) {
	s := newTestServer()
	do(s, http.MethodPost, "/_ctl", `{"fail_next":2,"fail_status":500,"fail_key":"sk-mock-a"}`, nil)
	other := map[string]string{"Authorization": "Bearer sk-mock-b"}
	target := map[string]string{"Authorization": "Bearer sk-mock-a"}
	if rec := do(s, http.MethodPost, "/v1/chat/completions", `{"model":"m"}`, other); rec.Code != http.StatusOK {
		t.Fatalf("other key status = %d, want 200", rec.Code)
	}
	for i := 0; i < 2; i++ {
		if rec := do(s, http.MethodPost, "/v1/chat/completions", `{"model":"m"}`, target); rec.Code != http.StatusInternalServerError {
			t.Fatalf("target key call %d status = %d, want 500", i, rec.Code)
		}
	}
	if rec := do(s, http.MethodPost, "/v1/chat/completions", `{"model":"m"}`, target); rec.Code != http.StatusOK {
		t.Fatalf("target key after budget status = %d, want 200", rec.Code)
	}
	do(s, http.MethodPost, "/_ctl", `{"reset":true}`, nil)
	if s.failKey != "" {
		t.Fatalf("failKey after reset = %q, want empty", s.failKey)
	}
}
