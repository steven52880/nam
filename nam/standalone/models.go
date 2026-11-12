package standalone

import (
	"context"
	"net/http"
)

// modelFetcher implements ports.ModelFetcher through NewAPI's own
// POST /api/channel/fetch_models (RootAuth), so Gemini/Ollama/OpenAI-style handling
// stays identical to the NewAPI UI.
type modelFetcher struct{ c *Client }

func (m modelFetcher) FetchModels(ctx context.Context, baseURL string, channelType int, key string) ([]string, error) {
	body := map[string]any{"base_url": baseURL, "type": channelType, "key": key}
	var out []string
	if err := m.c.call(ctx, http.MethodPost, "/api/channel/fetch_models", nil, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}
