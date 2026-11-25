// Package postgres initializes PostgreSQL connections for GORM.
package postgres

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Config is independent of the application's configuration loader.
// Zero pool lifetimes mean connections are not retired based on time.
// DSN is passed to the GORM PostgreSQL driver and must set a positive
// connect_timeout, for example: sslmode=disable&connect_timeout=5.
// connect_timeout bounds the startup dial; the caller's context bounds later
// queries.
type Config struct {
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

// Validate checks configuration without contacting PostgreSQL or exposing the DSN.
func (c Config) Validate() error {
	if c.DSN == "" {
		return errors.New("postgres: DSN is required")
	}
	parsed, err := pgx.ParseConfig(c.DSN)
	if err != nil {
		// Parser errors can include values from the DSN.
		return errors.New("postgres: invalid PostgreSQL DSN")
	}
	if parsed.Database == "" {
		return errors.New("postgres: database name is required")
	}
	if parsed.ConnectTimeout <= 0 {
		return errors.New("postgres: DSN must set positive connect_timeout")
	}
	if c.MaxOpenConns <= 0 || c.MaxIdleConns < 0 || c.MaxIdleConns > c.MaxOpenConns {
		return errors.New("postgres: require max_open_conns > 0 and 0 <= max_idle_conns <= max_open_conns")
	}
	if c.ConnMaxLifetime < 0 || c.ConnMaxIdleTime < 0 {
		return errors.New("postgres: connection lifetimes must not be negative")
	}
	return nil
}
