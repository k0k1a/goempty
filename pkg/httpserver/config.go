// Package httpserver manages HTTP server configuration and lifecycle.
package httpserver

import (
	"errors"
	"net"
	"strconv"
	"time"
)

// Config is independent of the application's configuration loader.
// Addr must be host:port; an empty host listens on all interfaces.
// Zero timeouts follow net/http defaults: ReadHeaderTimeout and IdleTimeout
// fall back to ReadTimeout; zero ReadTimeout and WriteTimeout mean no deadline.
type Config struct {
	Addr              string        `mapstructure:"addr"`
	ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout"`
	ReadTimeout       time.Duration `mapstructure:"read_timeout"`
	WriteTimeout      time.Duration `mapstructure:"write_timeout"`
	IdleTimeout       time.Duration `mapstructure:"idle_timeout"`
}

// Validate checks configuration without opening a listener.
func (c Config) Validate() error {
	_, port, err := net.SplitHostPort(c.Addr)
	if err != nil {
		return errors.New("httpserver: addr must be host:port")
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 0 || number > 65535 {
		return errors.New("httpserver: port must be between 0 and 65535")
	}
	if c.ReadHeaderTimeout < 0 || c.ReadTimeout < 0 || c.WriteTimeout < 0 || c.IdleTimeout < 0 {
		return errors.New("httpserver: timeouts must not be negative")
	}
	return nil
}
