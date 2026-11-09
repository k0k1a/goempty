// Package mongodb initializes shared MongoDB clients using the official driver.
package mongodb

import (
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const defaultMaxPoolSize = 100

// Config is independent of the application's configuration loader.
// Positive fields override URI options; zero retains URI options or driver defaults.
// Use maxPoolSize=0 in the URI to request an unlimited pool explicitly.
type Config struct {
	URI                    string        `mapstructure:"uri"`
	MaxPoolSize            int           `mapstructure:"max_pool_size"`
	MinPoolSize            int           `mapstructure:"min_pool_size"`
	MaxConnIdleTime        time.Duration `mapstructure:"max_conn_idle_time"`
	ConnectTimeout         time.Duration `mapstructure:"connect_timeout"`
	ServerSelectionTimeout time.Duration `mapstructure:"server_selection_timeout"`
	Timeout                time.Duration `mapstructure:"timeout"`
}

// Validate checks settings without connecting to MongoDB or exposing URI contents.
// The driver may resolve DNS for SRV URIs and read TLS files while parsing options.
func (c Config) Validate() error {
	_, err := c.clientOptions()
	return err
}

func (c Config) clientOptions() (*options.ClientOptions, error) {
	if strings.TrimSpace(c.URI) == "" {
		return nil, errors.New("mongodb: uri is required")
	}
	if c.MaxPoolSize < 0 || c.MinPoolSize < 0 {
		return nil, errors.New("mongodb: pool sizes must not be negative")
	}
	if c.MaxConnIdleTime < 0 || c.ConnectTimeout < 0 || c.ServerSelectionTimeout < 0 || c.Timeout < 0 {
		return nil, errors.New("mongodb: durations must not be negative")
	}
	opts := options.Client().ApplyURI(c.URI)
	if c.MaxPoolSize > 0 {
		opts.SetMaxPoolSize(uint64(c.MaxPoolSize))
	}
	if c.MinPoolSize > 0 {
		opts.SetMinPoolSize(uint64(c.MinPoolSize))
	}
	if c.MaxConnIdleTime > 0 {
		opts.SetMaxConnIdleTime(c.MaxConnIdleTime)
	}
	if c.ConnectTimeout > 0 {
		opts.SetConnectTimeout(c.ConnectTimeout)
	}
	if c.ServerSelectionTimeout > 0 {
		opts.SetServerSelectionTimeout(c.ServerSelectionTimeout)
	}
	if c.Timeout > 0 {
		opts.SetTimeout(c.Timeout)
	}
	if err := opts.Validate(); err != nil {
		// Driver parsing errors may contain credentials or URI query values.
		return nil, errors.New("mongodb: invalid URI or client options")
	}
	maxPoolSize := uint64(defaultMaxPoolSize)
	if opts.MaxPoolSize != nil {
		maxPoolSize = *opts.MaxPoolSize
	}
	if opts.MinPoolSize != nil && maxPoolSize > 0 && *opts.MinPoolSize > maxPoolSize {
		return nil, errors.New("mongodb: min_pool_size must not exceed max_pool_size")
	}
	return opts, nil
}
