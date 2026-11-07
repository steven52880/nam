// Command mockupstream is a minimal mock of a NewAPI-style upstream used by QA.
// It supports the OpenAI chat/completions, Anthropic messages and Gemini
// generateContent endpoints, plus a small control plane for fault injection.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	defaultFailureStatus = 500
	maxRecordedCalls     = 1000
	chunkCount           = 50
	tokensPerChunk       = 10
	firstTokenDelay      = 300 * time.Millisecond
	interChunkDelay      = 40 * time.Millisecond
)

// callRecord is one recorded relay request.
type callRecord struct {
	Time        int64  `json:"time"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	AuthHeader  string `json:"auth_header"`
	XAPIKey     string `json:"x_api_key"`
	XGoogAPIKey string `json:"x_goog_api_key"`
	Model       string `json:"model"`
	Stream      bool   `json:"stream"`
	Status      int    `json:"status"`
}

// server holds the mutable control state and recorded calls.
type server struct {
	mu           sync.Mutex
	failNext     int
	failStatus   int
	latencyScale float64
	calls        []callRecord
}

// newServer returns a server with defaults applied.
func newServer() *server {
	return &server{
		failStatus:   defaultFailureStatus,
		latencyScale: 1.0,
	}
}

// statusWriter captures the response status while delegating to the underlying writer.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) code() int {
	if w.status == 0 {
		return 200
	}
	return w.status
}

// ServeHTTP routes every request and records relay traffic.
func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rw := &statusWriter{ResponseWriter: w}
	start := time.Now()
	body, _ := io.ReadAll(r.Body)

	slog.Info("request", "method", r.Method, "path", r.URL.RequestURI())

	recorded := strings.HasPrefix(r.URL.Path, "/v1/") || strings.HasPrefix(r.URL.Path, "/v1beta/")
	model, stream := extractMeta(r, body)
	if recorded {
		// closure: rw.code() must be read after the handler wrote the status
		defer func() { s.record(start, r, model, stream, rw.code()) }()
	}

	if isRelayPath(r.URL.Path) {
		if status, ok := s.consumeFail(); ok {
			writeJSON(rw, status, map[string]any{
				"error": map[string]any{"message": "mock injected error", "type": "mock"},
			})
			return
		}
	}

	switch {
	case r.URL.Path == "/v1/chat/completions":
		s.handleChat(rw, r, body)
	case r.URL.Path == "/v1/messages":
		s.handleMessages(rw, r, body)
	case strings.HasPrefix(r.URL.Path, "/v1beta/models/"):
		s.handleGemini(rw, r, body)
	case r.URL.Path == "/v1/models" && r.Method == http.MethodGet:
		writeJSON(rw, http.StatusOK, map[string]any{
			"object": "list",
			"data": []any{
				map[string]any{"id": "test-model", "object": "model"},
				map[string]any{"id": "test-model-2", "object": "model"},
			},
		})
	case r.URL.Path == "/_ctl" && r.Method == http.MethodPost:
		s.handleCtl(rw, body)
	case r.URL.Path == "/_calls" && r.Method == http.MethodGet:
		writeJSON(rw, http.StatusOK, s.snapshotCalls())
	case r.URL.Path == "/_calls" && r.Method == http.MethodDelete:
		s.clearCalls()
		writeJSON(rw, http.StatusOK, map[string]any{"ok": true})
	default:
		writeJSON(rw, http.StatusNotFound, map[string]any{
			"error": map[string]any{"message": "not found", "type": "mock"},
		})
	}
}

func isRelayPath(p string) bool {
	return p == "/v1/chat/completions" || p == "/v1/messages" || strings.HasPrefix(p, "/v1beta/models/")
}

func extractMeta(r *http.Request, body []byte) (string, bool) {
	switch {
	case r.URL.Path == "/v1/chat/completions" || r.URL.Path == "/v1/messages":
		var req struct {
			Model  string `json:"model"`
			Stream bool   `json:"stream"`
		}
		_ = json.Unmarshal(body, &req)
		return req.Model, req.Stream
	case strings.HasPrefix(r.URL.Path, "/v1beta/models/"):
		model, action := splitGeminiPath(r.URL.Path)
		stream := strings.Contains(action, "stream") || r.URL.Query().Get("alt") == "sse"
		return model, stream
	}
	return "", false
}

func splitGeminiPath(path string) (model, action string) {
	rest := strings.TrimPrefix(path, "/v1beta/models/")
	if i := strings.Index(rest, ":"); i >= 0 {
		return rest[:i], rest[i+1:]
	}
	return rest, ""
}

func (s *server) consumeFail() (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failNext > 0 {
		s.failNext--
		return s.failStatus, true
	}
	return 0, false
}

func (s *server) getScale() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.latencyScale
}

func (s *server) sleep(d time.Duration) {
	time.Sleep(time.Duration(float64(d) * s.getScale()))
}

func (s *server) record(start time.Time, r *http.Request, model string, stream bool, status int) {
	xGoog := r.Header.Get("x-goog-api-key")
	if xGoog == "" {
		xGoog = r.URL.Query().Get("key")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, callRecord{
		Time:        start.UnixMilli(),
		Method:      r.Method,
		Path:        r.URL.RequestURI(),
		AuthHeader:  r.Header.Get("Authorization"),
		XAPIKey:     r.Header.Get("x-api-key"),
		XGoogAPIKey: xGoog,
		Model:       model,
		Stream:      stream,
		Status:      status,
	})
	if len(s.calls) > maxRecordedCalls {
		s.calls = s.calls[len(s.calls)-maxRecordedCalls:]
	}
}

func (s *server) snapshotCalls() []callRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]callRecord, len(s.calls))
	copy(out, s.calls)
	return out
}

func (s *server) clearCalls() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = nil
}

func (s *server) handleCtl(w http.ResponseWriter, body []byte) {
	var req struct {
		FailNext     *int     `json:"fail_next"`
		FailStatus   *int     `json:"fail_status"`
		LatencyScale *float64 `json:"latency_scale"`
		Reset        *bool    `json:"reset"`
	}
	_ = json.Unmarshal(body, &req)

	s.mu.Lock()
	if req.Reset != nil && *req.Reset {
		s.failNext = 0
		s.failStatus = defaultFailureStatus
		s.latencyScale = 1.0
		s.calls = nil
	}
	if req.FailNext != nil {
		s.failNext = *req.FailNext
	}
	if req.FailStatus != nil {
		s.failStatus = *req.FailStatus
	}
	if req.LatencyScale != nil {
		s.latencyScale = *req.LatencyScale
	}
	state := map[string]any{
		"fail_next":     s.failNext,
		"fail_status":   s.failStatus,
		"latency_scale": s.latencyScale,
	}
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, state)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func getFlusher(w http.ResponseWriter) http.Flusher {
	if f, ok := w.(http.Flusher); ok {
		return f
	}
	return nopFlusher{}
}

type nopFlusher struct{}

func (nopFlusher) Flush() {}

func setSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
}

func usageChat() map[string]any {
	return map[string]any{
		"prompt_tokens":         1000,
		"completion_tokens":     500,
		"total_tokens":          1500,
		"prompt_tokens_details": map[string]any{"cached_tokens": 200},
	}
}

func usageAnthropic() map[string]any {
	return map[string]any{
		"input_tokens":                800,
		"cache_read_input_tokens":     200,
		"cache_creation_input_tokens": 0,
		"output_tokens":               500,
	}
}

func usageGemini() map[string]any {
	return map[string]any{
		"promptTokenCount":        1000,
		"cachedContentTokenCount": 200,
		"candidatesTokenCount":    500,
		"totalTokenCount":         1500,
	}
}

func chunkText() string {
	return strings.Repeat("x ", tokensPerChunk)
}

func fullText() string {
	return strings.Repeat("x ", chunkCount*tokensPerChunk)
}

func writeEvent(w http.ResponseWriter, fl http.Flusher, name string, payload any) {
	b, _ := json.Marshal(payload)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, b)
	fl.Flush()
}

func writeData(w http.ResponseWriter, fl http.Flusher, payload any) {
	b, _ := json.Marshal(payload)
	fmt.Fprintf(w, "data: %s\n\n", b)
	fl.Flush()
}

func (s *server) handleChat(w http.ResponseWriter, r *http.Request, body []byte) {
	var req struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	_ = json.Unmarshal(body, &req)
	created := time.Now().Unix()

	if req.Stream {
		setSSEHeaders(w)
		fl := getFlusher(w)
		s.streamChat(w, fl, req.Model, created)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":      "chatcmpl-mock",
		"object":  "chat.completion",
		"created": created,
		"model":   req.Model,
		"choices": []any{
			map[string]any{
				"index":         0,
				"message":       map[string]any{"role": "assistant", "content": fullText()},
				"finish_reason": "stop",
			},
		},
		"usage": usageChat(),
	})
}

func (s *server) streamChat(w http.ResponseWriter, fl http.Flusher, model string, created int64) {
	s.sleep(firstTokenDelay)
	for i := 0; i < chunkCount; i++ {
		writeData(w, fl, map[string]any{
			"id":      "chatcmpl-mock",
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   model,
			"choices": []any{
				map[string]any{
					"index":         0,
					"delta":         map[string]any{"content": chunkText()},
					"finish_reason": nil,
				},
			},
		})
		if i < chunkCount-1 {
			s.sleep(interChunkDelay)
		}
	}
	writeData(w, fl, map[string]any{
		"id":      "chatcmpl-mock",
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   model,
		"choices": []any{
			map[string]any{
				"index":         0,
				"delta":         map[string]any{},
				"finish_reason": "stop",
			},
		},
		"usage": usageChat(),
	})
	fmt.Fprint(w, "data: [DONE]\n\n")
	fl.Flush()
}

func (s *server) handleMessages(w http.ResponseWriter, r *http.Request, body []byte) {
	var req struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	_ = json.Unmarshal(body, &req)

	if req.Stream {
		setSSEHeaders(w)
		fl := getFlusher(w)
		s.streamMessages(w, fl, req.Model)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":          "msg-mock",
		"type":        "message",
		"role":        "assistant",
		"content":     []any{map[string]any{"type": "text", "text": fullText()}},
		"model":       req.Model,
		"stop_reason": "end_turn",
		"usage":       usageAnthropic(),
	})
}

func (s *server) streamMessages(w http.ResponseWriter, fl http.Flusher, model string) {
	writeEvent(w, fl, "message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":      "msg-mock",
			"type":    "message",
			"role":    "assistant",
			"content": []any{},
			"model":   model,
			"usage": map[string]any{
				"input_tokens":                800,
				"cache_read_input_tokens":     200,
				"cache_creation_input_tokens": 0,
				"output_tokens":               0,
			},
		},
	})
	writeEvent(w, fl, "content_block_start", map[string]any{
		"type":          "content_block_start",
		"index":         0,
		"content_block": map[string]any{"type": "text", "text": ""},
	})

	s.sleep(firstTokenDelay)
	for i := 0; i < chunkCount; i++ {
		writeEvent(w, fl, "content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{"type": "text_delta", "text": chunkText()},
		})
		if i < chunkCount-1 {
			s.sleep(interChunkDelay)
		}
	}

	writeEvent(w, fl, "content_block_stop", map[string]any{
		"type":  "content_block_stop",
		"index": 0,
	})
	writeEvent(w, fl, "message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": "end_turn"},
		"usage": map[string]any{"output_tokens": 500},
	})
	writeEvent(w, fl, "message_stop", map[string]any{"type": "message_stop"})
}

func (s *server) handleGemini(w http.ResponseWriter, r *http.Request, body []byte) {
	_, action := splitGeminiPath(r.URL.Path)
	streaming := strings.Contains(action, "stream") || r.URL.Query().Get("alt") == "sse"

	if streaming {
		setSSEHeaders(w)
		fl := getFlusher(w)
		s.streamGemini(w, fl)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"candidates": []any{
			map[string]any{
				"content":      map[string]any{"parts": []any{map[string]any{"text": fullText()}}, "role": "model"},
				"finishReason": "STOP",
			},
		},
		"usageMetadata": usageGemini(),
	})
}

func (s *server) streamGemini(w http.ResponseWriter, fl http.Flusher) {
	s.sleep(firstTokenDelay)
	for i := 0; i < chunkCount; i++ {
		writeData(w, fl, map[string]any{
			"candidates": []any{
				map[string]any{
					"content": map[string]any{"parts": []any{map[string]any{"text": chunkText()}}, "role": "model"},
				},
			},
		})
		if i < chunkCount-1 {
			s.sleep(interChunkDelay)
		}
	}
	writeData(w, fl, map[string]any{
		"candidates": []any{
			map[string]any{
				"content":      map[string]any{"parts": []any{map[string]any{"text": ""}}, "role": "model"},
				"finishReason": "STOP",
			},
		},
		"usageMetadata": usageGemini(),
	})
}

func main() {
	addr := flag.String("addr", ":9100", "listen address")
	flag.Parse()

	slog.Info("mock upstream starting", "addr", *addr)
	if err := http.ListenAndServe(*addr, newServer()); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
