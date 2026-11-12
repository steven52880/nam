package cachetest

import (
	"encoding/json"
	"strings"
)

// Protocols.
const (
	ProtocolAnthropic = "anthropic" // POST /v1/messages
	ProtocolOpenAI    = "openai"    // POST /v1/chat/completions
)

// ProtocolFor picks the protocol from the model name: claude* → anthropic, else openai.
func ProtocolFor(model string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "claude") {
		return ProtocolAnthropic
	}
	return ProtocolOpenAI
}

// Path returns the NewAPI relay path of a protocol.
func Path(protocol string) string {
	if protocol == ProtocolAnthropic {
		return "/v1/messages"
	}
	return "/v1/chat/completions"
}

// Turn is one finished question/answer pair of the conversation history.
type Turn struct {
	Question string
	Answer   string
}

// QuestionFor returns the fixed question of round i (1-based, cycled).
func QuestionFor(i int) string {
	return questions[(i-1)%len(questions)]
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// BuildRound builds the request body of round i (1-based): corpus + history + question i.
// Anthropic: system is one text block (the corpus) carrying cache_control ephemeral.
// OpenAI: the corpus is the system message and relies on automatic prefix caching.
// Both use max_tokens 128, non-streaming. temperature is left at the model default:
// newer models (e.g. claude-sonnet-5-5) reject any non-default temperature with 400.
func BuildRound(protocol, model string, i int, history []Turn) ([]byte, error) {
	msgs := make([]message, 0, len(history)*2+1)
	for _, t := range history {
		msgs = append(msgs, message{Role: "user", Content: t.Question}, message{Role: "assistant", Content: t.Answer})
	}
	msgs = append(msgs, message{Role: "user", Content: QuestionFor(i)})

	if protocol == ProtocolAnthropic {
		return json.Marshal(map[string]any{
			"model":      model,
			"max_tokens": 128,
			"system": []map[string]any{{
				"type":          "text",
				"text":          corpus,
				"cache_control": map[string]string{"type": "ephemeral"},
			}},
			"messages": msgs,
		})
	}
	all := append([]message{{Role: "system", Content: corpus}}, msgs...)
	return json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": 128,
		"messages":   all,
	})
}
