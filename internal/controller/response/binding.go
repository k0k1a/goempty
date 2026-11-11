package response

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"goempty/internal/entity"
	"goempty/pkg/log"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

// A non-empty sentinel prevents validator from falling back to the Go field name.
const hiddenBindingField = "-"

var initBindingValidator = sync.OnceValue(func() error {
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return errors.New("HTTP binding requires go-playground/validator/v10")
	}
	v.RegisterTagNameFunc(bindingFieldName)
	return nil
})

// InitBindingValidator configures Gin's shared validator once, before any binding
// or validation occurs. Do not replace binding.Validator after initialization.
func InitBindingValidator() error {
	return initBindingValidator()
}

// validator caches this callback's result along with each struct's metadata.
func bindingFieldName(field reflect.StructField) string {
	jsonName := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
	formName := strings.SplitN(field.Tag.Get("form"), ",", 2)[0]
	if field.Anonymous || jsonName == "-" || formName == "-" ||
		(jsonName != "" && formName != "" && jsonName != formName) {
		return hiddenBindingField
	}
	if jsonName != "" {
		return jsonName
	}
	if formName != "" {
		return formName
	}
	return hiddenBindingField
}

// RespondBindError handles a non-nil error returned by Gin's ShouldBind methods.
// InitBindingValidator must run before validation. Callers must return afterward.
func RespondBindError(c *gin.Context, err error) {
	var invalidErr *validator.InvalidValidationError
	if err == nil || errors.As(err, &invalidErr) {
		log.Error(c.Request.Context(), "HTTP binding failed internally", zap.String("error_type", fmt.Sprintf("%T", err)))
		Failure(c, http.StatusInternalServerError, -1, "未知错误")
		return
	}

	message := entity.ErrInvalidArgument.Message
	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) && len(validationErrs) > 0 && validationErrs[0] != nil {
		fieldErr := validationErrs[0]
		// Indexed paths can contain user-controlled map keys. Hidden ancestors
		// also prevent exposing a nested field that is not part of the public input.
		namespace := "." + fieldErr.Namespace() + "."
		if fieldErr.Field() != "" && !strings.ContainsAny(namespace, "[]") &&
			!strings.Contains(namespace, "."+hiddenBindingField+".") {
			message = fieldErr.Field() + " 格式校验失败"
		}
	}
	Failure(c, http.StatusBadRequest, entity.ErrInvalidArgument.Code, message)
}
