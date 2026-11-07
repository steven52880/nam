package scheduler

import "errors"

var (
	ErrUnknownJob = errors.New("scheduler: unknown job")
	ErrQueueFull  = errors.New("scheduler: queue full")
)
