// Package nambridge is the embedded NewAPI adapter for nam. It is compiled into
// new-api and implements nam's ports on top of NewAPI's own model/controller code.
package nambridge

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/core"
	"github.com/steven52880/nam/ports"
)

const (
	envSecret   = "NAM_SECRET"
	envDBPath   = "NAM_DB_PATH"
	envDisabled = "NAM_DISABLED"
)

var (
	mu       sync.Mutex
	instance *core.Core
)

func newPorts() ports.Ports {
	return ports.Ports{
		Channels: channelStore{},
		Options:  optionStore{},
		Logs:     logSource{},
		Auth:     authAdapter{},
		Backend:  backendInfo{},
	}
}

func dbPath() string {
	if p := os.Getenv(envDBPath); p != "" {
		return p
	}
	if st, err := os.Stat("/data"); err == nil && st.IsDir() {
		return "/data/nam.db"
	}
	return "./data/nam.db"
}

// protect runs fn and turns a panic into an error so NewAPI keeps running.
func protect(name string, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			common.SysError(fmt.Sprintf("nam: panic in %s: %v", name, r))
			err = fmt.Errorf("nam: panic in %s: %v", name, r)
		}
	}()
	return fn()
}

// Mount registers /api/nam/* and /nam/* on engine. It must run before NewAPI's
// router so the static handler and NoRoute do not swallow nam's paths.
// A missing NAM_SECRET or NAM_DISABLED=true skips mounting without error.
// Errors are logged here and also returned.
func Mount(engine *gin.Engine) error {
	err := protect("Mount", func() error {
		if strings.EqualFold(os.Getenv(envDisabled), "true") {
			common.SysLog("nam: disabled by NAM_DISABLED")
			return nil
		}
		secret := os.Getenv(envSecret)
		if secret == "" {
			common.SysError("nam: NAM_SECRET is empty, nam is NOT mounted (NewAPI continues without it)")
			return nil
		}
		c, err := core.New(core.Config{
			DBPath: dbPath(),
			Secret: secret,
			PanicHook: func(name string, recovered any, _ []byte) {
				common.SysError(fmt.Sprintf("nam: recovered panic in %s: %v", name, recovered))
			},
		}, newPorts())
		if err != nil {
			return err
		}
		c.Mount(engine)
		mu.Lock()
		instance = c
		mu.Unlock()
		common.SysLog("nam: mounted at /nam/ and /api/nam/")
		return nil
	})
	if err != nil {
		common.SysError("nam: mount failed: " + err.Error())
	}
	return err
}

// Start launches nam's scheduler; it stops with common.ShutdownCtx(). No-op if not mounted.
func Start() {
	_ = protect("Start", func() error {
		mu.Lock()
		c := instance
		mu.Unlock()
		if c != nil {
			c.Start(common.ShutdownCtx())
		}
		return nil
	})
}

// Stop stops the scheduler and closes nam.db (mainly for tests; shutdown also stops it).
func Stop() {
	_ = protect("Stop", func() error {
		mu.Lock()
		c := instance
		instance = nil
		mu.Unlock()
		if c != nil {
			c.Stop()
		}
		return nil
	})
}
