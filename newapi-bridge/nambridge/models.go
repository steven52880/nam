package nambridge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/gin-gonic/gin"
)

// modelFetcher implements ports.ModelFetcher by driving controller.FetchModels (the handler
// behind POST /api/channel/fetch_models) through a synthetic gin context, so Gemini/Ollama/
// OpenAI-style handling is exactly what the NewAPI UI gets. The key only travels in the
// in-memory request body; it is never logged or echoed in errors.
type modelFetcher struct{}

type fetchModelsEnvelope struct {
	Success bool     `json:"success"`
	Message string   `json:"message"`
	Data    []string `json:"data"`
}

func (modelFetcher) FetchModels(ctx context.Context, baseURL string, channelType int, key string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	body, err := common.Marshal(map[string]any{"base_url": baseURL, "type": channelType, "key": key})
	if err != nil {
		return nil, err
	}
	rec := httptest.NewRecorder()
	gc, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/nam-internal/channel/fetch_models", bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	gc.Request = req
	if err := protect("fetch-models", func() error { controller.FetchModels(gc); return nil }); err != nil {
		return nil, err
	}
	var env fetchModelsEnvelope
	if err := common.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		return nil, fmt.Errorf("parse fetch_models response (HTTP %d): %w", rec.Code, err)
	}
	if !env.Success {
		msg := env.Message
		if msg == "" {
			msg = fmt.Sprintf("fetch models failed (HTTP %d)", rec.Code)
		}
		return nil, errors.New(msg)
	}
	return env.Data, nil
}
