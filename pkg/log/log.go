package log

import (
	"errors"
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New creates a logger. Stop logging and call Close at application shutdown.
// ErrorOutputPaths receives Zap internal errors, not application Error logs.
func New(cfg Config) (*zap.Logger, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		return nil, fmt.Errorf("log: parse level: %w", err)
	}
	outputs := newOutputs(cfg)
	output, err := outputs.open(cfg.OutputPaths)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("log: open output: %w", err), outputs.close())
	}
	errorOutput, err := outputs.open(cfg.ErrorOutputPaths)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("log: open error output: %w", err), outputs.close())
	}
	var encoder zapcore.Encoder
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	if cfg.Encoding == "console" {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}
	// No sampler: record every enabled entry.
	core := &managedCore{Core: zapcore.NewCore(encoder, output, level), close: outputs.close}
	return zap.New(core, zap.ErrorOutput(errorOutput), zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)), nil
}

// Close releases all owned file outputs once; stdout and stderr stay open.
// The root and its With/Named children share outputs. Stop all logging first.
func Close(logger *zap.Logger) error {
	if logger == nil {
		return errors.New("log: logger is required")
	}
	core, ok := logger.Core().(*managedCore)
	if !ok {
		return errors.New("log: logger must use the core created by New")
	}
	return core.close()
}

type managedCore struct {
	zapcore.Core
	close func() error
}

func (c *managedCore) With(fields []zapcore.Field) zapcore.Core {
	return &managedCore{Core: c.Core.With(fields), close: c.close}
}
