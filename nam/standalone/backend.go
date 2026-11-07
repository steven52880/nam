package standalone

import (
	"context"
	"net/http"

	"github.com/steven52880/nam/ports"
)

// backend implements ports.Backend using GET /api/status.
type backend struct {
	c   *Client
	url string
}

func (b backend) Info(ctx context.Context) (ports.BackendInfo, error) {
	var st struct {
		Version      string  `json:"version"`
		QuotaPerUnit float64 `json:"quota_per_unit"`
	}
	err := b.c.call(ctx, http.MethodGet, "/api/status", nil, nil, &st)
	at, lastErr := b.c.lastState()
	info := ports.BackendInfo{
		Mode:    ports.ModeStandalone,
		Address: b.url,
		// TODO: PatchP1 stays false until the standalone adapter can detect the
		// use_time_ms patch (e.g. from a log row's "other" field).
		PatchP1:       false,
		LastRequestAt: at,
		LastError:     lastErr,
	}
	if err != nil {
		return info, err
	}
	info.Version = st.Version
	info.QuotaPerUnit = st.QuotaPerUnit
	return info, nil
}
