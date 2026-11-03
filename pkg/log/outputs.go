package log

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type outputs struct {
	cfg      Config
	files    map[string]*lumberjack.Logger
	once     sync.Once
	closeErr error
}

func newOutputs(cfg Config) *outputs {
	return &outputs{cfg: cfg, files: make(map[string]*lumberjack.Logger)}
}

func (o *outputs) open(paths []string) (zapcore.WriteSyncer, error) {
	writers := make([]zapcore.WriteSyncer, 0, len(paths))
	for _, path := range paths {
		if path == "stdout" || path == "stderr" {
			writer, _, err := zap.Open(path)
			if err != nil {
				return nil, err
			}
			writers = append(writers, writer)
			continue
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		key := abs
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		file := o.files[key]
		if file == nil {
			file = &lumberjack.Logger{
				Filename:   abs,
				MaxSize:    o.cfg.MaxSize,
				MaxAge:     o.cfg.MaxAge,
				MaxBackups: o.cfg.MaxBackups,
				LocalTime:  true,
				Compress:   o.cfg.Compress,
			}
			o.files[key] = file
			// Open now so invalid paths fail at startup, not on the first log.
			if _, err := file.Write(nil); err != nil {
				return nil, err
			}
		}
		writers = append(writers, zapcore.AddSync(file))
	}
	return zap.CombineWriteSyncers(writers...), nil
}

func (o *outputs) close() error {
	o.once.Do(func() {
		for _, file := range o.files {
			o.closeErr = errors.Join(o.closeErr, file.Close())
		}
	})
	return o.closeErr
}
