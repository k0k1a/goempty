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
// The pool is configured and pinged once with ctx before New returns; a failed
// ping closes the pool and returns the error. Use WithContext for subsequent queries.
func New(ctx context.Context, cfg Config) (*gorm.DB, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("db: initialize: %w", err)
	}
	handle, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		// Driver initialization errors may echo the DSN.
		return nil, errors.New("db: invalid MySQL DSN")
	}
	return initialize(ctx, cfg, handle)
}

// SetLogger returns a session using log, sharing the original connection pool.
// The original handle is unchanged. A nil log retains its current logger.
func SetLogger(handle *gorm.DB, log logger.Interface) *gorm.DB {
	return handle.Session(&gorm.Session{Logger: log})
}

func initialize(ctx context.Context, cfg Config, handle *gorm.DB) (*gorm.DB, error) {
	pool, err := sqlPool(handle)
	if err != nil {
		return nil, err
	}
	pool.SetMaxOpenConns(cfg.MaxOpenConns)
	pool.SetMaxIdleConns(cfg.MaxIdleConns)
	pool.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	pool.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	if err := pool.PingContext(ctx); err != nil {
		_ = pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
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
