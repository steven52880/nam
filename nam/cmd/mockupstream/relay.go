package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

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
