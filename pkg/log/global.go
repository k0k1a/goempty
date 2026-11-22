package log

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"

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

// loggers holds two views of the same core. wrapped compensates for the package
// wrapper when reporting the caller; root reports direct calls unchanged.
type loggers struct {
	wrapped *zap.Logger
	root    *zap.Logger
	close   func() error
}

// nopLoggers replaces the default logger after Shutdown or a failed
// initialization, so later calls never reopen closed files.
var nopLoggers = &loggers{wrapped: zap.NewNop(), root: zap.NewNop()}

var (
	initMu  sync.Mutex
	closed  atomic.Bool
	state   atomic.Pointer[loggers]
	initErr error // guarded by initMu; set only when initialization failed.
)

// currentLoggers returns the shared loggers, creating them on first use.
func currentLoggers() *loggers {
	if l := state.Load(); l != nil {
		return l
	}
	return initializeLoggers()
}

func initializeLoggers() *loggers {
	initMu.Lock()
	defer initMu.Unlock()
	if l := state.Load(); l != nil {
		return l
	}
	if closed.Load() {
		state.Store(nopLoggers)
		return nopLoggers
	}
	root, err := newDefaultLogger()
	if err != nil {
		// Package functions cannot return errors. Surface a broken configuration
		// at first use instead of at import; recoveries keep using a no-op logger.
		initErr = err
		state.Store(nopLoggers)
		panic(err)
	}
	if closed.Load() {
		// Shutdown ran while the logger was being created; release it instead of
		// leaving file outputs open after shutdown.
		_ = Close(root)
		state.Store(nopLoggers)
		return nopLoggers
	}
	l := &loggers{
		wrapped: root.WithOptions(zap.AddCallerSkip(1)),
		root:    root,
		close:   func() error { return Close(root) },
	}
	state.Store(l)
	return l
}

func newDefaultLogger() (*zap.Logger, error) {
	return New(Config{
		Level:            env.Get("LEVEL", "info"),
		Encoding:         "json",
		OutputPaths:      []string{"stdout", filepath.Join("logs", env.AppName()+"_"+env.Env()+".log")},
		ErrorOutputPaths: []string{"stderr"},
		MaxSize:          100,
		MaxAge:           90,
		MaxBackups:       0,
		Compress:         true,
	})
}

// Zap returns the default logger as a *zap.Logger for integrations that need it
// directly. It shares the package logger's outputs and reports callers for
// direct calls. Do not Close or replace it; previously obtained values must not
// be used after Shutdown.
func Zap() *zap.Logger {
	return currentLoggers().root
}

// Shutdown closes the default logger's file outputs and makes later logging a
// no-op; it never recreates files. It returns nil when nothing was logged.
// Stop all logging first; do not log after shutdown. Repeated calls are safe.
func Shutdown() error {
	closed.Store(true)
	initMu.Lock()
	defer initMu.Unlock()
	l := state.Load()
	state.Store(nopLoggers)
	err := initErr
	if l != nil && l.close != nil {
		err = errors.Join(err, l.close())
	}
	return err
}

// Debug records a debug entry using the default logger.
func Debug(ctx context.Context, msg string, fields ...zap.Field) {
	currentLoggers().wrapped.Debug(msg, withFields(ctx, fields...)...)
}

// Info records an info entry using the default logger.
func Info(ctx context.Context, msg string, fields ...zap.Field) {
	currentLoggers().wrapped.Info(msg, withFields(ctx, fields...)...)
}

// Warn records a warning entry using the default logger.
func Warn(ctx context.Context, msg string, fields ...zap.Field) {
	currentLoggers().wrapped.Warn(msg, withFields(ctx, fields...)...)
}

// Error records an error entry using the default logger.
func Error(ctx context.Context, msg string, fields ...zap.Field) {
	currentLoggers().wrapped.Error(msg, withFields(ctx, fields...)...)
}
