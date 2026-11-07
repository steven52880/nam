// Command nam runs nam in standalone mode against a remote NewAPI.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/steven52880/nam/core"
	"github.com/steven52880/nam/internal/safego"
	"github.com/steven52880/nam/standalone"
)

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(core.Version)
		return
	}
	if err := run(); err != nil {
		slog.Error("nam: fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := standalone.FromEnv()
	if err != nil {
		return err
	}
	p, client, err := standalone.New(cfg)
	if err != nil {
		return err
	}
	c, err := core.New(core.Config{DBPath: cfg.DBPath, Secret: cfg.Secret}, p)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	c.Start(ctx)

	vctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	if err := client.VerifyRoot(vctx); err != nil {
		slog.Warn("nam: NewAPI root check failed (continuing)", "err", err)
	}
	cancel()

	srv := &http.Server{Addr: cfg.Listen, Handler: c.Handler(), ReadHeaderTimeout: 10 * time.Second}
	errCh := make(chan error, 1)
	safego.Go("nam-http", func() {
		slog.Info("nam: listening", "addr", cfg.Listen, "version", core.Version)
		errCh <- srv.ListenAndServe()
	})
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		slog.Info("nam: shutting down")
	}
	sctx, scancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer scancel()
	return srv.Shutdown(sctx)
}
