// Package store owns nam.db (SQLite via glebarez/sqlite + gorm, same driver as NewAPI).
package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// Store wraps the nam.db connection.
type Store struct {
	DB   *gorm.DB
	path string
}

// Open opens (or creates) nam.db with WAL and migrates all tables.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open nam.db: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	// SQLite: one writer. A single connection avoids SQLITE_BUSY between nam goroutines.
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(AllModels()...); err != nil {
		return nil, fmt.Errorf("migrate nam.db: %w", err)
	}
	return &Store{DB: db, path: path}, nil
}

// Close closes the database.
func (s *Store) Close() error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Path returns the database file path.
func (s *Store) Path() string { return s.path }

// Backup writes a consistent copy of nam.db to dst using VACUUM INTO.
func (s *Store) Backup(ctx context.Context, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	_ = os.Remove(dst)
	return s.DB.WithContext(ctx).Exec("VACUUM INTO ?", dst).Error
}

// GetKV implements ports.KV.
func (s *Store) GetKV(ctx context.Context, key string) (string, bool, error) {
	var st Setting
	err := s.DB.WithContext(ctx).First(&st, "key = ?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return st.Value, true, nil
}

// SetKV implements ports.KV.
func (s *Store) SetKV(ctx context.Context, key, value string) error {
	return s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&Setting{Key: key, Value: value}).Error
}

// GetFloat returns a numeric setting or def.
func (s *Store) GetFloat(ctx context.Context, key string, def float64) float64 {
	v, ok, err := s.GetKV(ctx, key)
	if err != nil || !ok {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

// GetBool returns a boolean setting or def.
func (s *Store) GetBool(ctx context.Context, key string, def bool) bool {
	v, ok, err := s.GetKV(ctx, key)
	if err != nil || !ok {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

// GetDuration returns a duration setting (stored as Go duration string) or def.
func (s *Store) GetDuration(ctx context.Context, key string, def time.Duration) time.Duration {
	v, ok, err := s.GetKV(ctx, key)
	if err != nil || !ok {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

// Setting keys shared across packages.
const (
	KeySiteRate           = "site_rate"            // 站点汇率，默认 7
	KeyProtectThreshold   = "protect_threshold"    // 变化保护阈值，默认 0.5
	KeyAutoWrite          = "auto_write"           // 自动写入开关
	KeyLogRetentionDays   = "log_retention_days"   // NewAPI 原始日志保留天数，默认 30
	KeyStatsRetentionDays = "stats_retention_days" // 聚合保留天数，默认 180
	KeyJobIntervalPrefix  = "job_interval."        // + job name
)

// Defaults applied on first start (only if absent).
var Defaults = map[string]string{
	KeySiteRate:           "7",
	KeyProtectThreshold:   "0.5",
	KeyAutoWrite:          "true",
	KeyLogRetentionDays:   "30",
	KeyStatsRetentionDays: "180",
	KeyStatsBackfillDays:  "7",
	KeyNewAPILogCleanup:   "false",
}

// EnsureDefaults inserts default settings that are missing.
func (s *Store) EnsureDefaults(ctx context.Context) error {
	for k, v := range Defaults {
		if _, ok, err := s.GetKV(ctx, k); err != nil {
			return err
		} else if !ok {
			if err := s.SetKV(ctx, k, v); err != nil {
				return err
			}
		}
	}
	return nil
}
