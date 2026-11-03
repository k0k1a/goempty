// Package log initializes structured Zap loggers.
package log

import (
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap/zapcore"
)

// Config is independent of the application's configuration loader.
// New uses these values as supplied without applying defaults.
// MaxAge and MaxBackups of zero disable their respective retention limits.
type Config struct {
	Level            string   `mapstructure:"level"`
	Encoding         string   `mapstructure:"encoding"`
	OutputPaths      []string `mapstructure:"output_paths"`
	ErrorOutputPaths []string `mapstructure:"error_output_paths"`
	MaxSize          int      `mapstructure:"max_size"` // Megabytes per file.
	MaxAge           int      `mapstructure:"max_age"`  // Days to retain backups.
	MaxBackups       int      `mapstructure:"max_backups"`
	Compress         bool     `mapstructure:"compress"`
}

// Validate checks configuration without opening output paths.
func (c Config) Validate() error {
	if c.MaxSize <= 0 || c.MaxAge < 0 || c.MaxBackups < 0 {
		return errors.New("log: max_size must be positive and retention limits must not be negative")
	}
	if c.Level == "" {
		return errors.New("log: level is required")
	}
	if _, err := zapcore.ParseLevel(c.Level); err != nil {
		return fmt.Errorf("log: invalid level: %w", err)
	}
	if c.Encoding != "json" && c.Encoding != "console" {
		return errors.New("log: encoding must be json or console")
	}
	for _, paths := range [][]string{c.OutputPaths, c.ErrorOutputPaths} {
		if len(paths) == 0 {
			return errors.New("log: output_paths and error_output_paths are required")
		}
		for _, path := range paths {
			if strings.TrimSpace(path) == "" {
				return errors.New("log: output paths must not contain blank entries")
			}
		}
	}
	return nil
}
