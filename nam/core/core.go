// Package core is the public entry point of nam. Both adapters call New and mount
// the returned handlers; nothing else in nam is imported by them.
package core

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/internal/api"
	"github.com/steven52880/nam/internal/audit"
	"github.com/steven52880/nam/internal/cachetest"
	"github.com/steven52880/nam/internal/channels"
	"github.com/steven52880/nam/internal/crypto"
	"github.com/steven52880/nam/internal/safego"
	"github.com/steven52880/nam/internal/scheduler"
	"github.com/steven52880/nam/internal/store"
	"github.com/steven52880/nam/internal/upstream"
	"github.com/steven52880/nam/internal/vendors"
	"github.com/steven52880/nam/ports"
	"github.com/steven52880/nam/webdist"
)

const notBuiltPage = `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><title>nam</title></head>` +
	`<body><p>nam 前端尚未构建：在 web/ 下执行 npm run build，再重新编译。</p></body></html>`

// Version is overridden at build time with -ldflags.
var Version = "0.1.0-dev"

// Config configures the core.
type Config struct {
	DBPath    string // NAM_DB_PATH
	Secret    string // NAM_SECRET
	BackupDir string // default: <dir of DBPath>/backup
	// WebFS overrides the embedded frontend (useful for dev). Nil uses webdist.
	WebFS fs.FS
	// PanicHook is called on recovered background panics (e.g. log to NewAPI SysError).
	PanicHook safego.PanicHook
	// PriceBookURL overrides the LiteLLM price list URL (tests). Empty = BerriAI's default.
	PriceBookURL string
}

// Core is a running nam instance.
type Core struct {
	cfg    Config
	store  *store.Store
	sched  *scheduler.Scheduler
	server *api.Server
	audit  *audit.Recorder
	ports  ports.Ports

	channels *channels.Service
	syncer   *upstream.Syncer
}

// New opens nam.db, wires services and returns a Core. Call Start to run jobs.
func New(cfg Config, p ports.Ports) (*Core, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if cfg.DBPath == "" {
		return nil, errors.New("core: DBPath is empty (set NAM_DB_PATH)")
	}
	box, err := crypto.New(cfg.Secret)
	if err != nil {
		return nil, err
	}
	if cfg.PanicHook != nil {
		safego.SetPanicHook(cfg.PanicHook)
	}
	if cfg.BackupDir == "" {
		cfg.BackupDir = filepath.Join(filepath.Dir(cfg.DBPath), "backup")
	}
	if cfg.WebFS == nil {
		cfg.WebFS = webdist.FS()
	}
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	if err := st.EnsureDefaults(context.Background()); err != nil {
		_ = st.Close()
		return nil, err
	}
	if b, okKV := p.Auth.(ports.KVBinder); okKV {
		b.BindKV(st)
	}
	if b, okKV := p.Channels.(ports.KVBinder); okKV {
		b.BindKV(st)
	}
	rec := audit.New(st.DB)
	wrapped := p
	wrapped.Channels = rec.Channels(p.Channels)
	wrapped.Options = rec.Options(p.Options)
	wrapped.Models = rec.Models(p.Models)

	sch := scheduler.New(st)
	c := &Core{cfg: cfg, store: st, sched: sch, audit: rec, ports: wrapped}
	c.channels = &channels.Service{St: st, Box: box, Channels: wrapped.Channels}
	c.syncer = &upstream.Syncer{St: st, Box: box, HTTP: &http.Client{Timeout: 30 * time.Second}, Audit: rec}
	c.server = &api.Server{Deps: api.Deps{
		Store: st, Box: box, Audit: rec, Sched: sch, Ports: wrapped, Version: Version,
		Channels: c.channels, Syncer: c.syncer, Vendors: &vendors.Service{St: st},
		CacheTest: &cachetest.Runner{St: st, Box: box, Channels: wrapped.Channels, Backend: wrapped.Backend, Audit: rec},
	}}
	c.registerJobs()
	return c, nil
}

// registerJobs registers every background job. Bodies not yet implemented are no-ops so the
// scheduler, UI and API contract are stable.
func (c *Core) registerJobs() {
	noop := func(string) scheduler.Func {
		return func(ctx context.Context) (string, error) { return "未实现", nil }
	}
	c.sched.Register(scheduler.Job{Name: scheduler.JobBalance, Title: "余额同步", Default: 30 * time.Minute, Fn: c.balanceJob})
	c.sched.Register(scheduler.Job{Name: scheduler.JobRatio, Title: "倍率同步", Default: 6 * time.Hour, Fn: c.ratioJob})
	c.sched.Register(scheduler.Job{Name: scheduler.JobPriceBook, Title: "价格库同步", Default: 24 * time.Hour, Fn: c.priceBookJob})
	c.sched.Register(scheduler.Job{Name: scheduler.JobAnnouncements, Title: "公告检查", Default: 6 * time.Hour, Fn: noop("announcements")})
	c.sched.Register(scheduler.Job{Name: scheduler.JobPullLogs, Title: "日志拉取", Default: 2 * time.Minute, Fn: noop("pull_logs")})
	c.sched.Register(scheduler.Job{Name: scheduler.JobPricing, Title: "定价计算", Default: 30 * time.Minute, Fn: noop("pricing")})
	c.sched.Register(scheduler.Job{Name: scheduler.JobConsistency, Title: "一致性检查", Default: 10 * time.Minute, Fn: c.consistencyJob})
	c.sched.Register(scheduler.Job{Name: scheduler.JobCleanup, Title: "日志清理", Default: 24 * time.Hour, Fn: noop("cleanup")})
	c.sched.Register(scheduler.Job{Name: scheduler.JobBackup, Title: "备份", Default: 24 * time.Hour, Fn: c.backupJob})
}

func (c *Core) backupJob(ctx context.Context) (string, error) {
	dst := filepath.Join(c.cfg.BackupDir, "nam-"+time.Now().Format("20060102")+".db")
	if err := c.store.Backup(ctx, dst); err != nil {
		return "", err
	}
	return dst, nil
}

// Start runs the scheduler until ctx is cancelled (embedded: common.ShutdownCtx()).
func (c *Core) Start(ctx context.Context) {
	c.sched.Start(ctx)
	safego.Go("nam-stop-on-ctx", func() {
		<-ctx.Done()
		c.Stop()
	})
}

// Stop stops jobs and closes nam.db. Safe to call more than once.
func (c *Core) Stop() {
	c.sched.Stop()
	if c.store != nil {
		if err := c.store.Close(); err != nil {
			slog.Warn("nam: close db", "err", err)
		}
	}
}

// Mount registers /api/nam/* and /nam/* on an existing gin engine.
// Explicit routes take precedence over NewAPI's NoRoute fallback.
func (c *Core) Mount(r gin.IRouter) {
	c.server.Register(r.Group("/api/nam"))
	web := http.FS(c.cfg.WebFS)
	serve := func(gc *gin.Context) {
		p := strings.TrimPrefix(gc.Param("filepath"), "/")
		if p != "" {
			if f, err := c.cfg.WebFS.Open(p); err == nil {
				st, _ := f.Stat()
				_ = f.Close()
				if st != nil && !st.IsDir() {
					if strings.HasPrefix(p, "assets/") {
						gc.Header("Cache-Control", "public, max-age=31536000, immutable")
					}
					gc.FileFromFS(p, web)
					return
				}
			}
		}
		// SPA fallback
		b, err := fs.ReadFile(c.cfg.WebFS, "index.html")
		if err != nil {
			// fresh checkout: only dist/.gitkeep is committed
			b = []byte(notBuiltPage)
		}
		gc.Header("Cache-Control", "no-cache")
		gc.Data(http.StatusOK, "text/html; charset=utf-8", b)
	}
	r.GET("/nam", func(gc *gin.Context) { gc.Redirect(http.StatusMovedPermanently, "/nam/") })
	r.GET("/nam/*filepath", serve)
	r.HEAD("/nam/*filepath", serve)
}

// Handler returns a standalone gin engine with nam mounted (standalone mode).
func (c *Core) Handler() http.Handler {
	gin.SetMode(gin.ReleaseMode)
	e := gin.New()
	e.Use(gin.Recovery())
	e.GET("/healthz", func(gc *gin.Context) { gc.String(http.StatusOK, "ok") })
	e.GET("/", func(gc *gin.Context) { gc.Redirect(http.StatusFound, "/nam/") })
	c.Mount(e)
	return e
}

// KV exposes nam.db's key/value store (adapters may persist small state).
func (c *Core) KV() ports.KV { return c.store }
