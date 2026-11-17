// Command mockupstream is a minimal mock of a NewAPI-style upstream used by QA.
// It supports the OpenAI chat/completions, Anthropic messages and Gemini
// generateContent endpoints, plus a small control plane for fault injection.
package main

import (
	"encoding/json"
	"flag"
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
	failKey      string // when set, fail_next only applies to requests carrying this key
	latencyScale float64
	calls        []callRecord

	// Management API state (NewAPI-style upstream simulation).
	quota           int
	accessToken     string
	userID          int
	username        string
	password        string
	notice          string
	groupRatio      map[string]float64
	modelRatio      map[string]float64
	completionRatio map[string]float64
	tokens          []tokenState
	nextTokenID     int
	customBalance   json.RawMessage
}

// newServer returns a server with defaults applied.
func newServer() *server {
	s := &server{
		failStatus:   defaultFailureStatus,
		latencyScale: 1.0,
	}
	s.resetMgmt()
	return s
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

	recorded := strings.HasPrefix(r.URL.Path, "/v1/") ||
		strings.HasPrefix(r.URL.Path, "/v1beta/") ||
		strings.HasPrefix(r.URL.Path, "/api/") ||
		strings.HasPrefix(r.URL.Path, "/_custom/")
	model, stream := extractMeta(r, body)
	if recorded {
		// closure: rw.code() must be read after the handler wrote the status
		defer func() { s.record(start, r, model, stream, rw.code()) }()
	}

	if isRelayPath(r.URL.Path) {
		if status, ok := s.consumeFail(r); ok {
			writeJSON(rw, status, map[string]any{
				"error": map[string]any{"message": "mock injected error", "type": "mock"},
			})
			return
		}
	}

	switch {
	case isManagementPath(r.URL.Path):
		s.handleManagement(rw, r, body)
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

func (s *server) consumeFail(r *http.Request) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failKey != "" && !requestHasKey(r, s.failKey) {
		return 0, false
	}
	if s.failNext > 0 {
		s.failNext--
		return s.failStatus, true
	}
	return 0, false
}

// requestHasKey reports whether the request carries key in any of the supported auth headers.
func requestHasKey(r *http.Request, key string) bool {
	if strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") == key {
		return true
	}
	if r.Header.Get("x-api-key") == key || r.Header.Get("x-goog-api-key") == key {
		return true
	}
	return r.URL.Query().Get("key") == key
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
		FailNext        *int                `json:"fail_next"`
		FailStatus      *int                `json:"fail_status"`
		FailKey         *string             `json:"fail_key"`
		LatencyScale    *float64            `json:"latency_scale"`
		Reset           *bool               `json:"reset"`
		Quota           *int                `json:"quota"`
		GroupRatio      *map[string]float64 `json:"group_ratio"`
		ModelRatio      *map[string]float64 `json:"model_ratio"`
		CompletionRatio *map[string]float64 `json:"completion_ratio"`
		Notice          *string             `json:"notice"`
		Tokens          *[]tokenState       `json:"tokens"`
		CustomBalance   json.RawMessage     `json:"custom_balance"`
		AccessToken     *string             `json:"access_token"`
	}
	_ = json.Unmarshal(body, &req)

	s.mu.Lock()
	if req.Reset != nil && *req.Reset {
		s.failNext = 0
		s.failStatus = defaultFailureStatus
		s.failKey = ""
		s.latencyScale = 1.0
		s.calls = nil
		s.resetMgmt()
	}
	if req.FailNext != nil {
		s.failNext = *req.FailNext
	}
	if req.FailStatus != nil {
		s.failStatus = *req.FailStatus
	}
	if req.FailKey != nil {
		s.failKey = *req.FailKey
	}
	if req.LatencyScale != nil {
		s.latencyScale = *req.LatencyScale
	}
	if req.Quota != nil {
		s.quota = *req.Quota
	}
	if req.GroupRatio != nil {
		s.groupRatio = *req.GroupRatio
	}
	if req.ModelRatio != nil {
		s.modelRatio = *req.ModelRatio
	}
	if req.CompletionRatio != nil {
		s.completionRatio = *req.CompletionRatio
	}
	if req.Notice != nil {
		s.notice = *req.Notice
	}
	if req.Tokens != nil {
		s.tokens = *req.Tokens
		s.nextTokenID = maxTokenID(s.tokens) + 1
	}
	if len(req.CustomBalance) > 0 {
		s.customBalance = append(json.RawMessage(nil), req.CustomBalance...)
	}
	if req.AccessToken != nil {
		s.accessToken = *req.AccessToken
	}
	state := map[string]any{
		"fail_next":        s.failNext,
		"fail_status":      s.failStatus,
		"fail_key":         s.failKey,
		"latency_scale":    s.latencyScale,
		"quota":            s.quota,
		"access_token":     s.accessToken,
		"user_id":          s.userID,
		"username":         s.username,
		"notice":           s.notice,
		"group_ratio":      s.groupRatio,
		"model_ratio":      s.modelRatio,
		"completion_ratio": s.completionRatio,
		"tokens":           s.tokens,
		"custom_balance":   json.RawMessage(s.customBalance),
	}
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, state)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
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
