// Package standalone implements the nam ports against a remote NewAPI over its
// admin HTTP API, plus a single-admin password login for the nam UI.
package standalone

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config is the standalone runtime configuration (all NAM_* environment variables).
type Config struct {
	NewAPIURL     string // NAM_NEWAPI_URL
	Token         string // NAM_NEWAPI_TOKEN: root system access token
	UserID        int    // NAM_NEWAPI_USER_ID: id of the root user owning the token
	AdminPassword string // NAM_ADMIN_PASSWORD: initial password, used only until a hash is stored
	Listen        string // NAM_LISTEN
	DBPath        string // NAM_DB_PATH
	Secret        string // NAM_SECRET
	CookieSecure  bool   // NAM_COOKIE_SECURE
}

// FromEnv reads the configuration from the environment and validates it.
func FromEnv() (Config, error) {
	cfg := Config{
		NewAPIURL:     strings.TrimSpace(os.Getenv("NAM_NEWAPI_URL")),
		Token:         strings.TrimSpace(os.Getenv("NAM_NEWAPI_TOKEN")),
		AdminPassword: os.Getenv("NAM_ADMIN_PASSWORD"),
		Listen:        envOr("NAM_LISTEN", "127.0.0.1:8090"),
		DBPath:        envOr("NAM_DB_PATH", "./data/nam.db"),
		Secret:        os.Getenv("NAM_SECRET"),
	}
	if raw := strings.TrimSpace(os.Getenv("NAM_NEWAPI_USER_ID")); raw != "" {
		id, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("NAM_NEWAPI_USER_ID %q is not an integer: %w", raw, err)
		}
		cfg.UserID = id
	}
	if raw := strings.TrimSpace(os.Getenv("NAM_COOKIE_SECURE")); raw != "" {
		secure, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("NAM_COOKIE_SECURE %q is not a bool: %w", raw, err)
		}
		cfg.CookieSecure = secure
	}
	if cfg.Secret == "" {
		return Config{}, errors.New("NAM_SECRET is required")
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate checks the fields the NewAPI adapter needs.
func (c Config) Validate() error {
	switch {
	case c.NewAPIURL == "":
		return errors.New("NAM_NEWAPI_URL is required")
	case c.Token == "":
		return errors.New("NAM_NEWAPI_TOKEN is required")
	case c.UserID <= 0:
		return errors.New("NAM_NEWAPI_USER_ID is required (positive integer)")
	}
	return nil
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
