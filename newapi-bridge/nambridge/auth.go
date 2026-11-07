package nambridge

import (
	"context"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/ports"
)

// authAdapter implements ports.Auth using NewAPI's own session / access-token auth.
type authAdapter struct{}

func (authAdapter) Mode() string { return ports.ModeEmbedded }

// Middleware is NewAPI's RootAuth: only role 100 passes (session or access token +
// New-Api-User header).
//
// NewAPI rejects insufficient privilege / invalid token with HTTP 200 + success=false.
// nam's frontend needs a real status to tell "not root" apart from a handler error, so
// a 200 written before authHelper has set "id" is rewritten to 403. No buffering: the
// wrapper only changes the status code, streaming and hijacking are untouched.
func (authAdapter) Middleware() gin.HandlerFunc {
	root := middleware.RootAuth()
	return func(c *gin.Context) {
		c.Writer = &rejectStatusWriter{ResponseWriter: c.Writer, c: c}
		root(c)
	}
}

type rejectStatusWriter struct {
	gin.ResponseWriter
	c *gin.Context
}

func (w *rejectStatusWriter) WriteHeader(code int) {
	if code == http.StatusOK {
		if _, authed := w.c.Get("id"); !authed {
			code = http.StatusForbidden
		}
	}
	w.ResponseWriter.WriteHeader(code)
}

// SecureMiddleware reuses NewAPI's 2FA/Passkey secure verification (5 minute window).
func (authAdapter) SecureMiddleware() gin.HandlerFunc { return middleware.SecureVerificationRequired() }

// Identity reads the keys authHelper stores on the gin context ("id", "username", "role").
func (authAdapter) Identity(c *gin.Context) (ports.Identity, bool) {
	id := c.GetInt("id")
	if id == 0 {
		return ports.Identity{}, false
	}
	return ports.Identity{UserID: id, Username: c.GetString("username"), Role: c.GetInt("role")}, true
}

// backendInfo implements ports.Backend.
type backendInfo struct{}

func (backendInfo) Info(ctx context.Context) (ports.BackendInfo, error) {
	return ports.BackendInfo{
		Mode:         ports.ModeEmbedded,
		Version:      common.Version,
		QuotaPerUnit: common.QuotaPerUnit,
	}, nil
}
