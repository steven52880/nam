package standalone

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/steven52880/nam/ports"
)

const (
	cookieName     = "nam_session"
	sessionTTL     = 7 * 24 * time.Hour
	kvHashKey      = "standalone.admin_hash"
	kvSessionsKey  = "standalone.sessions"
	maxFailures    = 5
	lockDuration   = 60 * time.Second
	identityCtxKey = "nam.standalone.identity"
	adminUsername  = "admin"
)

// Auth is the single-admin password login. It implements ports.Auth,
// ports.RouteRegistrar and ports.KVBinder.
type Auth struct {
	userID    int
	secure    bool
	initialPW string
	now       func() time.Time

	mu          sync.Mutex
	kv          ports.KV
	memHash     string           // used only while no KV is bound
	sessions    map[string]int64 // sha256(token) hex -> expiry (unix seconds)
	loaded      bool
	failures    int
	lockedUntil int64
}

// NewAuth builds the auth component; the KV arrives later through BindKV.
func NewAuth(cfg Config) *Auth {
	return &Auth{
		userID: cfg.UserID, secure: cfg.CookieSecure, initialPW: cfg.AdminPassword,
		now: time.Now, sessions: map[string]int64{},
	}
}

var (
	_ ports.Auth           = (*Auth)(nil)
	_ ports.RouteRegistrar = (*Auth)(nil)
	_ ports.KVBinder       = (*Auth)(nil)
)

// BindKV hands the persistent store to the auth component (called by core.New).
func (a *Auth) BindKV(kv ports.KV) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.kv = kv
	a.loaded = false
}

func (a *Auth) Mode() string { return ports.ModeStandalone }

// SecureMiddleware is a pass-through in standalone mode.
// TODO: require a recent password re-entry before sensitive actions (credential reveal).
func (a *Auth) SecureMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) { c.Next() }
}

func (a *Auth) Identity(c *gin.Context) (ports.Identity, bool) {
	v, ok := c.Get(identityCtxKey)
	if !ok {
		return ports.Identity{}, false
	}
	id, ok := v.(ports.Identity)
	return id, ok
}

// Middleware accepts only requests carrying a valid, unexpired session cookie.
func (a *Auth) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(cookieName)
		if err != nil || token == "" || !a.validSession(c.Request.Context(), token) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录"})
			return
		}
		c.Set(identityCtxKey, ports.Identity{UserID: a.userID, Username: adminUsername, Role: rootRole})
		c.Next()
	}
}

func tokenKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// loadLocked reads persisted sessions once per KV binding. Caller holds a.mu.
func (a *Auth) loadLocked(ctx context.Context) {
	if a.loaded || a.kv == nil {
		return
	}
	a.loaded = true
	raw, ok, err := a.kv.GetKV(ctx, kvSessionsKey)
	if err != nil {
		slog.Warn("nam standalone: load sessions", "err", err)
		return
	}
	if !ok || raw == "" {
		return
	}
	var stored map[string]int64
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		slog.Warn("nam standalone: decode sessions", "err", err)
		return
	}
	for k, exp := range stored {
		a.sessions[k] = exp
	}
	a.pruneLocked()
}

func (a *Auth) pruneLocked() {
	now := a.now().Unix()
	for k, exp := range a.sessions {
		if exp <= now {
			delete(a.sessions, k)
		}
	}
}

func (a *Auth) persistLocked(ctx context.Context) {
	if a.kv == nil {
		return
	}
	b, err := json.Marshal(a.sessions)
	if err == nil {
		err = a.kv.SetKV(ctx, kvSessionsKey, string(b))
	}
	if err != nil {
		slog.Warn("nam standalone: persist sessions", "err", err)
	}
}

func (a *Auth) validSession(ctx context.Context, token string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.loadLocked(ctx)
	key := tokenKey(token)
	exp, ok := a.sessions[key]
	if !ok {
		return false
	}
	if exp <= a.now().Unix() {
		delete(a.sessions, key)
		a.persistLocked(ctx)
		return false
	}
	return true
}

// newSessionLocked mints a token, stores its hash and returns the token. Caller holds a.mu.
func (a *Auth) newSessionLocked(ctx context.Context) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	a.loadLocked(ctx)
	a.pruneLocked()
	a.sessions[tokenKey(token)] = a.now().Add(sessionTTL).Unix()
	a.persistLocked(ctx)
	return token, nil
}

func (a *Auth) setCookie(c *gin.Context, token string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: cookieName, Value: token, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode,
	})
}
