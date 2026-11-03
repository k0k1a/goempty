package controller

import (
	"errors"
	"net/http"

	"goempty/internal/controller/response"
	"goempty/internal/service"
	"goempty/pkg/log"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RespondError resolves wrapped business errors and writes a common error response.
// Callers must return after calling it. Unrecognized errors fall back to HTTP 500.
func RespondError(c *gin.Context, err error) {
	var businessErr *service.Error
	if errors.As(err, &businessErr) && businessErr != nil {
		if status, ok := businessStatus(businessErr); ok {
			response.Failure(c, status, businessErr.Code, businessErr.Message)
			return
		}
	}
	log.Error(c.Request.Context(), "HTTP request failed", zap.Error(err))
	response.Failure(c, http.StatusInternalServerError, -1, "未知错误")
}

func businessStatus(err error) (int, bool) {
	switch {
	case errors.Is(err, service.ErrInvalidArgument):
		return http.StatusBadRequest, true
	case errors.Is(err, service.ErrUnauthenticated):
		return http.StatusUnauthorized, true
	case errors.Is(err, service.ErrForbidden):
		return http.StatusForbidden, true
	case errors.Is(err, service.ErrNotFound):
		return http.StatusNotFound, true
	case errors.Is(err, service.ErrConflict):
		return http.StatusConflict, true
	case errors.Is(err, service.ErrUnprocessable):
		return http.StatusUnprocessableEntity, true
	case errors.Is(err, service.ErrRateLimited):
		return http.StatusTooManyRequests, true
	case errors.Is(err, service.ErrServiceUnavailable):
		return http.StatusServiceUnavailable, true
	default:
		return 0, false
	}
}

// RouteNotFound handles requests that do not match a registered route.
func RouteNotFound(c *gin.Context) {
	response.Failure(c, http.StatusNotFound, service.ErrNotFound.Code, "接口不存在")
}

// MethodNotAllowed handles unsupported methods on registered routes.
func MethodNotAllowed(c *gin.Context) {
	response.Failure(c, http.StatusMethodNotAllowed, service.ErrInvalidArgument.Code, "请求方法不支持")
}
