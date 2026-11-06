package response

import (
	"errors"
	"net/http"

	"goempty/internal/entity"
	"goempty/pkg/log"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RespondError resolves wrapped business errors and writes a common error response.
// Callers must return after calling it. Unrecognized errors fall back to HTTP 500.
func RespondError(c *gin.Context, err error) {
	var businessErr *entity.Error
	if errors.As(err, &businessErr) && businessErr != nil {
		if status, ok := businessStatus(businessErr); ok {
			Failure(c, status, businessErr.Code, businessErr.Message)
			return
		}
	}
	log.Error(c.Request.Context(), "HTTP request failed", zap.Error(err))
	Failure(c, http.StatusInternalServerError, -1, "未知错误")
}

func businessStatus(err *entity.Error) (int, bool) {
	switch err.Code {
	case entity.CodeBusinessError:
		return http.StatusUnprocessableEntity, true
	case entity.ErrInvalidArgument.Code:
		return http.StatusBadRequest, true
	case entity.ErrUnauthenticated.Code:
		return http.StatusUnauthorized, true
	case entity.ErrForbidden.Code:
		return http.StatusForbidden, true
	case entity.ErrNotFound.Code:
		return http.StatusNotFound, true
	case entity.ErrConflict.Code:
		return http.StatusConflict, true
	case entity.ErrUnprocessable.Code:
		return http.StatusUnprocessableEntity, true
	case entity.ErrRateLimited.Code:
		return http.StatusTooManyRequests, true
	case entity.ErrServiceUnavailable.Code:
		return http.StatusServiceUnavailable, true
	default:
		return 0, false
	}
}
