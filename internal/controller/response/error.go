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

func businessStatus(err error) (int, bool) {
	switch {
	case errors.Is(err, entity.ErrInvalidArgument):
		return http.StatusBadRequest, true
	case errors.Is(err, entity.ErrUnauthenticated):
		return http.StatusUnauthorized, true
	case errors.Is(err, entity.ErrForbidden):
		return http.StatusForbidden, true
	case errors.Is(err, entity.ErrNotFound):
		return http.StatusNotFound, true
	case errors.Is(err, entity.ErrConflict):
		return http.StatusConflict, true
	case errors.Is(err, entity.ErrUnprocessable):
		return http.StatusUnprocessableEntity, true
	case errors.Is(err, entity.ErrRateLimited):
		return http.StatusTooManyRequests, true
	case errors.Is(err, entity.ErrServiceUnavailable):
		return http.StatusServiceUnavailable, true
	default:
		return 0, false
	}
}
