// Package safego is the only allowed way to start goroutines in nam.
//
// In embedded mode nam shares a process with the NewAPI gateway; gin's recovery
// only protects request handlers, so an unrecovered panic in a background
// goroutine would kill the gateway. Never write a bare `go func`.
package safego

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"sync/atomic"
)

// PanicHook is called after a recovered panic (e.g. to send an alert). Must not panic.
type PanicHook func(name string, recovered any, stack []byte)

var hook atomic.Pointer[PanicHook]

// SetPanicHook installs a global hook; pass nil to clear.
func SetPanicHook(h PanicHook) {
	if h == nil {
		hook.Store(nil)
		return
	}
	hook.Store(&h)
}

// Go runs fn in a new goroutine and recovers panics.
func Go(name string, fn func()) {
	go func() {
		defer Recover(name)
		fn()
	}()
}

// GoWG is Go plus WaitGroup accounting.
func GoWG(wg *sync.WaitGroup, name string, fn func()) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer Recover(name)
		fn()
	}()
}

// Run calls fn synchronously and converts a panic into an error.
func Run(name string, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			report(name, r)
			err = fmt.Errorf("panic in %s: %v", name, r)
		}
	}()
	return fn()
}

// Recover must be deferred directly.
func Recover(name string) {
	if r := recover(); r != nil {
		report(name, r)
	}
}

func report(name string, r any) {
	stack := debug.Stack()
	slog.Error("nam: recovered panic", "goroutine", name, "panic", r, "stack", string(stack))
	if h := hook.Load(); h != nil {
		func() {
			defer func() { _ = recover() }()
			(*h)(name, r, stack)
		}()
	}
}
