package log

import (
	"context"

	"go.uber.org/zap"
)

// withFields appends the request and trace IDs carried by ctx to fields.
// Missing keys, empty values and non-string values are omitted.
func withFields(ctx context.Context, fields ...zap.Field) []zap.Field {
	if ctx == nil {
		return fields
	}
	if id, _ := ctx.Value("request_id").(string); id != "" {
		fields = append(fields, zap.String("request_id", id))
	}
	if id, _ := ctx.Value("trace_id").(string); id != "" {
		fields = append(fields, zap.String("trace_id", id))
	}
	return fields
}
