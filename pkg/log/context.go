package log

import (
	"context"

	"go.uber.org/zap"
)

type requestIDKey struct{}

// WithRequestID returns a child context carrying the request ID for global logs.
// It does not generate or validate IDs. An empty ID omits the request_id field.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

func loggerForContext(ctx context.Context) *zap.Logger {
	if ctx != nil {
		if id, _ := ctx.Value(requestIDKey{}).(string); id != "" {
			return defaultLogger.With(zap.String("request_id", id))
		}
	}
	return defaultLogger
}
