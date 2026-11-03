package middleware

import (
	"fmt"
	"goempty/internal/controller/response"
	"goempty/pkg/log"
	"io"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AccessLog records the final response after Recovery has handled any panic.
// Register it before Recovery. Route is the matched template (empty if unmatched),
// avoiding raw paths, query parameters, request bodies and headers.
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		duration := time.Since(started)
		log.Info(c.Request.Context(), "HTTP request completed", zap.String("method", c.Request.Method),
			zap.String("route", c.FullPath()), zap.Int("status", c.Writer.Status()), zap.Int64("duration_us", duration.Microseconds()))
	}
}

// Recovery logs panics without dumping request headers or bodies.
// A response already sent to the client cannot be replaced with a JSON error.
func Recovery() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, recovered any) {
		if c.Writer.Written() {
			log.Error(c.Request.Context(), "HTTP panic after response started",
				zap.String("panic", fmt.Sprint(recovered)), zap.ByteString("stack", debug.Stack()))
			c.Abort()
			return
		}
		log.Error(c.Request.Context(), "HTTP request failed",
			zap.Error(fmt.Errorf("HTTP panic: %v", recovered)), zap.ByteString("stack", debug.Stack()))
		response.Failure(c, http.StatusInternalServerError, -1, "未知错误")
	})
}

// RequestID uses gin-contrib's default ID handling and attaches the ID to logs.
func RequestID() gin.HandlerFunc {
	return requestid.New(requestid.WithHandler(func(c *gin.Context, id string) {
		c.Request = c.Request.WithContext(log.WithRequestID(c.Request.Context(), id))
	}))
}
