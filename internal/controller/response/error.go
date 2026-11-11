package response

import (
	"errors"

	"goempty/internal/entity"
	"goempty/pkg/log"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RespondError resolves wrapped business errors and writes a common error response.
// Callers must return after calling it. Unrecognized errors use a generic message.
func RespondError(c *gin.Context, err error) {
	var businessErr *entity.Error
	if errors.As(err, &businessErr) && businessErr != nil {
		Failure(c, businessErr.Code, businessErr.Message)
		return
	}
	log.Error(c.Request.Context(), "HTTP request failed", zap.Error(err))
	Failure(c, -1, "未知错误")
}
