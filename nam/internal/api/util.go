package api

import (
	"context"
	"errors"
	"time"

	"github.com/steven52880/nam/internal/upstream"
)

func contextTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, d)
}

func errIsUnsupported(err error) bool { return errors.Is(err, upstream.ErrUnsupported) }
