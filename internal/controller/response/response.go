// Package response defines the common JSON response envelope.
package response

import "github.com/gin-gonic/gin"

const CodeOK = 0

// Response is the envelope for ordinary JSON endpoints.
// Data is always present; errors and empty results use null.
type Response[T any] struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      T      `json:"data"`
	RequestID string `json:"request_id,omitempty"` // RequestID is reserved and currently omitted; use the X-Request-ID header.
}

// Success writes a successful JSON response. Use c.Status directly for HTTP 204.
func Success[T any](c *gin.Context, status int, data T) {
	c.JSON(status, Response[T]{
		Code:    CodeOK,
		Message: "success",
		Data:    data,
	})
}

// Failure writes an error response and prevents subsequent handlers from running.
// The caller must still return from its current handler after calling Failure.
func Failure(c *gin.Context, status, code int, message string) {
	c.AbortWithStatusJSON(status, Response[any]{
		Code:    code,
		Message: message,
		Data:    nil,
	})
}
