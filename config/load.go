package config

import (
	"context"
	"fmt"
	"path/filepath"

	"goempty/pkg/env"
	"goempty/pkg/log"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Load reads configs/{APP_NAME}_{ENV}.yaml relative to the working directory.
// Empty or unset APP_NAME and ENV default to app and dev. The file is required.
// Call once at startup. File changes are decoded and published through Get;
// the returned value is only the initial snapshot. The watcher lives until exit.
func Load() (Config, error) {
	path, err := filepath.Abs(filepath.Join("configs", env.AppName()+"_"+env.Env()+".yaml"))
	if err != nil {
		return Config{}, fmt.Errorf("config: resolve path: %w", err)
	}
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("config: read %s: %w", path, err)
	}
	var cfg Config
	if err := v.UnmarshalExact(&cfg); err != nil {
		return Config{}, fmt.Errorf("config: decode %s: %w", path, err)
	}
	set(cfg)

	// Viper reads the file before invoking this callback. Business code only
	// accesses the protected snapshot through Get, never this Viper instance.
	v.OnConfigChange(func(_ fsnotify.Event) {
		var next Config
		if err := v.UnmarshalExact(&next); err != nil {
			log.Error(context.Background(), "Configuration reload failed", zap.Error(err))
			return
		}
		set(next)
		log.Info(context.Background(), "Configuration reloaded", zap.String("path", path))
	})
	v.WatchConfig()
	return cfg, nil
}
