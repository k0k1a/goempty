package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"goempty/internal/service"

	"github.com/gin-gonic/gin"
)

func TestRespondError(t *testing.T) {
	var nilBusinessError *service.Error
	for _, tc := range []struct {
		name    string
		err     error
		status  int
		code    int
		message string
	}{
		{"invalid argument", service.ErrInvalidArgument, 400, 100001, "请求参数不合法"},
		{"unauthenticated", service.ErrUnauthenticated, 401, 100002, "请先登录"},
		{"forbidden", service.ErrForbidden, 403, 100003, "无权限"},
		{"wrapped", fmt.Errorf("lookup failed: %w", service.ErrNotFound), 404, 100004, "资源不存在"},
		{"conflict", service.ErrConflict, 409, 100005, "状态冲突"},
		{"business rule", service.ErrUnprocessable, 422, 100006, "不满足业务规则"},
		{"rate limit", service.ErrRateLimited, 429, 100007, "请求过于频繁"},
		{"unavailable", service.ErrServiceUnavailable, 503, 100008, "服务暂时不可用"},
		{"unexpected", errors.New("private database detail"), 500, -1, "未知错误"},
		{"unregistered", service.NewError(999999, "private detail"), 500, -1, "未知错误"},
		{"same code and message different instance", service.NewError(service.ErrNotFound.Code, service.ErrNotFound.Message), 500, -1, "未知错误"},
		{"typed nil", nilBusinessError, 500, -1, "未知错误"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := gin.New()
			handler.Use(func(c *gin.Context) {
				RespondError(c, tc.err)
			})
			handler.GET("/", func(c *gin.Context) { t.Error("handler ran after error response") })
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
			var body map[string]json.RawMessage
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			var code int
			var message string
			if err := json.Unmarshal(body["code"], &code); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(body["message"], &message); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != tc.status || code != tc.code || message != tc.message || string(body["data"]) != "null" || len(body) != 3 {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestBusinessStatusWrappedError(t *testing.T) {
	err := fmt.Errorf("lookup failed: %w", service.ErrNotFound)
	status, ok := businessStatus(err)
	if !ok || status != http.StatusNotFound {
		t.Fatalf("businessStatus(wrapped ErrNotFound) = %d, %v", status, ok)
	}
}
