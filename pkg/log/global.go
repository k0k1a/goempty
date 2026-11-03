package log

import (
	"context"
	"path/filepath"

	"goempty/pkg/env"

	"go.uber.org/zap"
)

// Logger is the structured logging interface that components can depend on.
// Implementations must support concurrent logging calls.
type Logger interface {
	Debug(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
}

// defaultLogger is created once at package initialization and never replaced.
var defaultLogger = newDefaultLogger()

func newDefaultLogger() *zap.Logger {
	// Must makes unexpected initialization failure visible at process startup.
	root := zap.Must(New(Config{
		Level:            env.Get("LEVEL", "info"),
		Encoding:         "json",
		OutputPaths:      []string{"stdout", filepath.Join("logs", env.AppName()+"_"+env.Env()+".log")},
		ErrorOutputPaths: []string{"stderr"},
		MaxSize:          100,
		MaxAge:           90,
		MaxBackups:       0,
		Compress:         true,
	}))
	return root.WithOptions(zap.AddCallerSkip(1))
}

// Shutdown closes the default logger's file outputs without replacing it.
// Stop all logging first; do not log after shutdown. Repeated calls are safe.
func Shutdown() error {
	return Close(defaultLogger)
}

// Debug records a debug entry using the global logger.
func Debug(ctx context.Context, msg string, fields ...zap.Field) {
	loggerForContext(ctx).Debug(msg, fields...)
}

// Info records an info entry using the global logger.
func Info(ctx context.Context, msg string, fields ...zap.Field) {
	loggerForContext(ctx).Info(msg, fields...)
}

// Warn records a warning entry using the global logger.
func Warn(ctx context.Context, msg string, fields ...zap.Field) {
	loggerForContext(ctx).Warn(msg, fields...)
}

// Error records an error entry using the global logger.
func Error(ctx context.Context, msg string, fields ...zap.Field) {
	loggerForContext(ctx).Error(msg, fields...)
}
