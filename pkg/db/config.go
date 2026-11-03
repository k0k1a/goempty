// Package db initializes MySQL connections for GORM.
package db

import (
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
)

// Config is independent of the application's configuration loader.
// Zero pool lifetimes mean connections are not retired based on time.
// DSN must enable parseTime and set positive timeout, readTimeout, writeTimeout
// values, for example: parseTime=true&timeout=5s&readTimeout=5s&writeTimeout=5s.
// These bound individual network operations, not the total initialization time.
type Config struct {
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

// Validate checks configuration without contacting MySQL or exposing the DSN.
func (c Config) Validate() error {
	if c.DSN == "" {
		return errors.New("db: DSN is required")
	}
	parsed, err := mysql.ParseDSN(c.DSN)
	if err != nil {
		// Parser errors can include values from the DSN.
		return errors.New("db: invalid MySQL DSN")
	}
	if parsed.DBName == "" {
		return errors.New("db: database name is required")
	}
	if !parsed.ParseTime {
		return errors.New("db: DSN must enable parseTime")
	}
	if parsed.Timeout <= 0 || parsed.ReadTimeout <= 0 || parsed.WriteTimeout <= 0 {
		return errors.New("db: DSN must set positive timeout, readTimeout and writeTimeout")
	}
	if c.MaxOpenConns <= 0 || c.MaxIdleConns < 0 || c.MaxIdleConns > c.MaxOpenConns {
		return errors.New("db: require max_open_conns > 0 and 0 <= max_idle_conns <= max_open_conns")
	}
	if c.ConnMaxLifetime < 0 || c.ConnMaxIdleTime < 0 {
		return errors.New("db: connection lifetimes must not be negative")
	}
	return nil
}
