package response

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"goempty/internal/entity"

	"github.com/gin-gonic/gin"
)

func TestRespondError(t *testing.T) {
	var nilBusinessError *entity.Error
	for _, tc := range []struct {
		name    string
		err     error
		status  int
		code    int
		message string
	}{
		{"invalid argument", entity.ErrInvalidArgument, 400, 100001, "请求参数不合法"},
		{"unauthenticated", entity.ErrUnauthenticated, 400, 100002, "请先登录"},
		{"forbidden", entity.ErrForbidden, 400, 100003, "无权限"},
		{"wrapped", fmt.Errorf("lookup failed: %w", entity.ErrNotFound), 400, 100004, "资源不存在"},
		{"conflict", entity.ErrConflict, 400, 100005, "状态冲突"},
		{"business rule", entity.ErrUnprocessable, 400, 100006, "不满足业务规则"},
		{"rate limit", entity.ErrRateLimited, 400, 100007, "请求过于频繁"},
		{"unavailable", entity.ErrServiceUnavailable, 400, 100008, "服务暂时不可用"},
		{"unexpected", errors.New("private database detail"), 400, -1, "未知错误"},
		{"custom business code", entity.NewError(999999, "自定义业务错误"), 400, 999999, "自定义业务错误"},
		{"same code different instance", entity.NewError(entity.ErrNotFound.Code, "订单不存在"), 400, 100004, "订单不存在"},
		{"public business message", entity.NewBusinessError("库存不足"), 400, -1, "库存不足"},
		{"wrapped business message", fmt.Errorf("validation: %w", entity.NewBusinessError("格式无效")), 400, -1, "格式无效"},
		{"typed nil", nilBusinessError, 400, -1, "未知错误"},
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
