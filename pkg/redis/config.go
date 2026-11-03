// Package redis initializes standalone Redis clients.
package redis

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"time"
)

// Config is independent of the application's configuration loader.
// Zero pool and timeout values use go-redis defaults.
type Config struct {
	Addr         string        `mapstructure:"addr"`
	Username     string        `mapstructure:"username"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	PoolTimeout  time.Duration `mapstructure:"pool_timeout"`
}

// Validate checks configuration without contacting Redis or exposing credentials.
func (c Config) Validate() error {
	host, port, err := net.SplitHostPort(c.Addr)
	if err != nil || strings.TrimSpace(host) == "" || strings.ContainsAny(host, " \t\r\n/@") {
		return errors.New("redis: addr must be host:port")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return errors.New("redis: port must be between 1 and 65535")
	}
	if c.DB < 0 {
		return errors.New("redis: db must not be negative")
	}
	if c.PoolSize < 0 || c.MinIdleConns < 0 || (c.PoolSize > 0 && c.MinIdleConns > c.PoolSize) {
		return errors.New("redis: pool sizes must not be negative and min_idle_conns must not exceed an explicit pool_size")
	}
	if c.DialTimeout < 0 || c.ReadTimeout < 0 || c.WriteTimeout < 0 || c.PoolTimeout < 0 {
		return errors.New("redis: timeouts must not be negative")
	}
	return nil
}
