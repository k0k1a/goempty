// Package response defines the common JSON response envelope.
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const CodeOK = 0

// Response is the envelope for ordinary JSON endpoints.
// Data is always present; errors use null.
type Response[T any] struct {
	Code      int    `json:"code"`                 // 业务状态码，0 表示成功，非 0 表示失败；未知内部错误为 -1
	Message   string `json:"message"`              // 响应消息，成功时为 success，失败时为可公开的错误说明
	Data      T      `json:"data"`                 // 接口业务数据；失败时为 null
	RequestID string `json:"request_id,omitempty"` // 预留的可选请求 ID，当前不输出此字段；实际请求 ID 通过 X-Request-ID 响应头返回
}

// Success writes a successful JSON response with HTTP 200. Use c.Status directly for HTTP 204.
func Success[T any](c *gin.Context, data T) {
	c.JSON(http.StatusOK, Response[T]{
		Code:    CodeOK,
		Message: "success",
		Data:    data,
	})
}

// Failure writes an HTTP 400 error response and prevents subsequent handlers from running.
// The caller must still return from its current handler after calling Failure.
func Failure(c *gin.Context, code int, message string) {
	c.AbortWithStatusJSON(http.StatusBadRequest, Response[any]{
		Code:    code,
		Message: message,
		Data:    nil,
	})
}
