package log

import (
	"context"

	"go.uber.org/zap"
)

func loggerForContext(ctx context.Context) *zap.Logger {
	logger := defaultLogger
	if ctx != nil {
		for _, key := range []string{"request_id", "trace_id"} {
			if id, _ := ctx.Value(key).(string); id != "" {
				logger = logger.With(zap.String(key, id))
			}
		}
	}
	return logger
}
