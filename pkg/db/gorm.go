package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// New creates a shared root handle. The caller owns it and must call Close.
// ctx is checked before initialization; DSN timeouts bound network operations
// during gorm.Open, rather than ctx. Use WithContext for subsequent queries.
func New(ctx context.Context, cfg Config) (*gorm.DB, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("db: initialize: %w", err)
	}
	handle, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{})
	if err != nil {
		// GORM closes its pool when MySQL initialization or automatic ping fails.
		return nil, fmt.Errorf("db: initialize GORM: %w", err)
	}
	return initialize(cfg, handle)
}

// SetLogger returns a session using log, sharing the original connection pool.
// The original handle is unchanged. A nil log retains its current logger.
func SetLogger(handle *gorm.DB, log logger.Interface) *gorm.DB {
	return handle.Session(&gorm.Session{Logger: log})
}

func initialize(cfg Config, handle *gorm.DB) (*gorm.DB, error) {
	pool, err := sqlPool(handle)
	if err != nil {
		return nil, err
	}
	pool.SetMaxOpenConns(cfg.MaxOpenConns)
	pool.SetMaxIdleConns(cfg.MaxIdleConns)
	pool.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	pool.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	return handle, nil
}

// Ping checks connectivity using the caller's cancellation and deadline.
func Ping(ctx context.Context, handle *gorm.DB) error {
	pool, err := sqlPool(handle)
	if err != nil {
		return err
	}
	if err := pool.PingContext(ctx); err != nil {
		return fmt.Errorf("db: ping: %w", err)
	}
	return nil
}

// Close releases the shared connection pool. Call it once at app shutdown.
func Close(handle *gorm.DB) error {
	pool, err := sqlPool(handle)
	if err != nil {
		return err
	}
	if err := pool.Close(); err != nil {
		return fmt.Errorf("db: close: %w", err)
	}
	return nil
}

func sqlPool(handle *gorm.DB) (*sql.DB, error) {
	if handle == nil {
		return nil, errors.New("db: handle is required")
	}
	pool, err := handle.DB()
	if err != nil {
		return nil, fmt.Errorf("db: access pool: %w", err)
	}
	return pool, nil
}
