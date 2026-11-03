// Package env provides shared environment variable defaults.
package env

import "os"

var (
	appName = Get("APP_NAME", "app")
	env     = Get("ENV", "dev")
)

// Get returns the variable's value, or fallback when unset or empty.
// Values are read on each call without validation or caching.
func Get(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// AppName returns APP_NAME read at package initialization, defaulting to app.
func AppName() string {
	return appName
}

// Env returns ENV read at package initialization, defaulting to dev.
func Env() string {
	return env
}
