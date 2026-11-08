// Package resp holds the shared JSON envelope helpers for /api/nam handlers.
// Every feature package (api, channels, upstream, ...) uses it so responses stay uniform.
package resp

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/ports"
)

// Envelope is the uniform response shape (same as NewAPI).
type Envelope struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ValidationError → HTTP 400.
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

// ConflictError → HTTP 409.
type ConflictError struct{ Msg string }

func (e *ConflictError) Error() string { return e.Msg }

// UpstreamError → HTTP 502 (NewAPI or an upstream site failed).
type UpstreamError struct{ Err error }

func (e *UpstreamError) Error() string { return e.Err.Error() }
func (e *UpstreamError) Unwrap() error { return e.Err }

// Invalid returns a ValidationError.
func Invalid(msg string) error { return &ValidationError{Msg: msg} }

// Conflict returns a ConflictError.
func Conflict(msg string) error { return &ConflictError{Msg: msg} }

// Upstream wraps err as an UpstreamError (nil stays nil).
func Upstream(err error) error {
	if err == nil {
		return nil
	}
	return &UpstreamError{Err: err}
}

// OK writes {success:true, data}.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{Success: true, Data: data})
}

// Fail writes {success:false} with an explicit status.
func Fail(c *gin.Context, status int, err error) {
	msg := "error"
	if err != nil {
		msg = err.Error()
	}
	c.AbortWithStatusJSON(status, Envelope{Success: false, Message: msg})
}

// FailData is Fail with a data payload (e.g. per-item results).
func FailData(c *gin.Context, status int, msg string, data any) {
	c.AbortWithStatusJSON(status, Envelope{Success: false, Message: msg, Data: data})
}

// Status maps an error to its HTTP status.
func Status(err error) int {
	var ve *ValidationError
	var ce *ConflictError
	var ue *UpstreamError
	switch {
	case errors.As(err, &ve):
		return http.StatusBadRequest
	case errors.As(err, &ce):
		return http.StatusConflict
	case errors.Is(err, ports.ErrNotFound), errors.Is(err, store.ErrNotFound):
		return http.StatusNotFound
	case errors.As(err, &ue):
		return http.StatusBadGateway
	}
	return http.StatusInternalServerError
}

// FailFrom writes err with the status from Status.
func FailFrom(c *gin.Context, err error) { Fail(c, Status(err), err) }

// BindJSON binds the body or writes a 400. Returns false on failure.
func BindJSON(c *gin.Context, v any) bool {
	if err := c.ShouldBindJSON(v); err != nil {
		Fail(c, http.StatusBadRequest, Invalid("请求体不是合法 JSON："+err.Error()))
		return false
	}
	return true
}
