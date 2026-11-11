package response

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

func TestRespondBindError(t *testing.T) {
	if err := InitBindingValidator(); err != nil {
		t.Fatal(err)
	}
	type requiredReq struct {
		Name string `json:"name,omitempty" form:"name" binding:"required"`
	}
	type rangeReq struct {
		Age   int      `json:"age" form:"age" binding:"min=18,max=100"`
		Name  string   `json:"name" binding:"min=2,max=3"`
		Items []string `json:"items" binding:"min=1,max=2"`
	}
	type exactReq struct {
		Name  string   `json:"name" binding:"len=2"`
		Items []string `json:"items" binding:"len=2"`
	}
	type emailReq struct {
		Email string `json:"email" binding:"email"`
	}
	type item struct {
		Name string `json:"name" binding:"required"`
	}
	type nestedReq struct {
		Items []*item `json:"items" binding:"dive"`
	}
	type objectReq struct {
		Item *item `json:"item"`
	}
	type matrixReq struct {
		Items [][]item `json:"items" binding:"dive,dive"`
	}
	type mapReq struct {
		Items map[string]item `json:"items" binding:"dive"`
	}
	type unknownRuleReq struct {
		Name string `json:"name" binding:"startswith=public"`
	}
	type untaggedReq struct {
		InternalName string `binding:"required"`
	}
	type hiddenReq struct {
		InternalName string `json:"-" binding:"required"`
	}
	type nestedHiddenReq struct {
		InternalItem item
	}
	type boolReq struct {
		Enabled bool `json:"enabled" binding:"required"`
	}
	type pointerReq struct {
		Enabled *bool `json:"enabled" binding:"required"`
	}
	type numberReq struct {
		Count int `json:"count" binding:"required"`
	}
	type pointerNumberReq struct {
		Count *int `json:"count" binding:"required"`
	}
	type pointerRangeReq struct {
		Age *int `json:"age" binding:"required,min=18"`
	}
	type formReq struct {
		Name string `form:"name" binding:"required"`
	}
	type conflictingReq struct {
		Name string `json:"name" form:"user_name" binding:"required"`
	}

	const validRange = `{"age":20,"name":"你好","items":["a"]}`
	for _, tc := range []struct {
		name    string
		request any
		body    string
		query   string
		wrap    bool
		message string // Empty means binding succeeds.
	}{
		{name: "required", request: &requiredReq{}, body: `{}`, message: "name 格式校验失败"},
		{name: "first error", request: &rangeReq{}, body: `{}`, message: "age 格式校验失败"},
		{name: "numeric min", request: &rangeReq{}, body: `{"age":17,"name":"ab","items":["a"]}`, message: "age 格式校验失败"},
		{name: "numeric max", request: &rangeReq{}, body: `{"age":101,"name":"ab","items":["a"]}`, message: "age 格式校验失败"},
		{name: "string min", request: &rangeReq{}, body: `{"age":20,"name":"你","items":["a"]}`, message: "name 格式校验失败"},
		{name: "string max", request: &rangeReq{}, body: `{"age":20,"name":"你好世界","items":["a"]}`, message: "name 格式校验失败"},
		{name: "slice min", request: &rangeReq{}, body: `{"age":20,"name":"ab","items":[]}`, message: "items 格式校验失败"},
		{name: "slice max", request: &rangeReq{}, body: `{"age":20,"name":"ab","items":["a","b","c"]}`, message: "items 格式校验失败"},
		{name: "string len", request: &exactReq{}, body: `{"name":"a","items":["a","b"]}`, message: "name 格式校验失败"},
		{name: "slice len", request: &exactReq{}, body: `{"name":"ab","items":[]}`, message: "items 格式校验失败"},
		{name: "email hides value", request: &emailReq{}, body: `{"email":"private-token"}`, message: "email 格式校验失败"},
		{name: "nested slice", request: &nestedReq{}, body: `{"items":[{"name":"ok"},{}]}`, message: "请求参数不合法"},
		{name: "nested object", request: &objectReq{}, body: `{"item":{}}`, message: "name 格式校验失败"},
		{name: "nested slices", request: &matrixReq{}, body: `{"items":[[{}]]}`, message: "请求参数不合法"},
		{name: "map hides key", request: &mapReq{}, body: `{"items":{"private-token":{}}}`, message: "请求参数不合法"},
		{name: "other rule returns tag", request: &unknownRuleReq{}, body: `{"name":"private-token"}`, message: "name 格式校验失败"},
		{name: "missing tag", request: &untaggedReq{}, body: `{}`, message: "请求参数不合法"},
		{name: "hidden tag", request: &hiddenReq{}, body: `{}`, message: "请求参数不合法"},
		{name: "untagged parent", request: &nestedHiddenReq{}, body: `{}`, message: "请求参数不合法"},
		{name: "wrapped validation", request: &requiredReq{}, body: `{}`, wrap: true, message: "name 格式校验失败"},
		{name: "query same field name", request: &requiredReq{}, query: "/?name=", message: "name 格式校验失败"},
		{name: "query form only", request: &formReq{}, query: "/?name=", message: "name 格式校验失败"},
		{name: "conflicting json tags", request: &conflictingReq{}, body: `{}`, message: "请求参数不合法"},
		{name: "conflicting form tags", request: &conflictingReq{}, query: "/?user_name=", message: "请求参数不合法"},
		{name: "query invalid integer", request: &rangeReq{}, query: "/?age=private-token", message: "请求参数不合法"},
		{name: "json syntax", request: &requiredReq{}, body: `{"name":"private-token"`, message: "请求参数不合法"},
		{name: "json type", request: &rangeReq{}, body: `{"age":"private-token"}`, message: "请求参数不合法"},
		{name: "json empty", request: &requiredReq{}, message: "请求参数不合法"},
		{name: "json null", request: &requiredReq{}, body: `null`, message: "name 格式校验失败"},
		{name: "required false", request: &boolReq{}, body: `{"enabled":false}`, message: "enabled 格式校验失败"},
		{name: "required pointer absent", request: &pointerReq{}, body: `{}`, message: "enabled 格式校验失败"},
		{name: "required zero", request: &numberReq{}, body: `{"count":0}`, message: "count 格式校验失败"},
		{name: "required pointer zero absent", request: &pointerNumberReq{}, body: `{}`, message: "count 格式校验失败"},
		{name: "pointer range", request: &pointerRangeReq{}, body: `{"age":17}`, message: "age 格式校验失败"},
		{name: "valid unicode length", request: &rangeReq{}, body: validRange},
		{name: "pointer false accepted", request: &pointerReq{}, body: `{"enabled":false}`},
		{name: "pointer zero accepted", request: &pointerNumberReq{}, body: `{"count":0}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := gin.New()
			handler.Use(func(c *gin.Context) {
				var err error
				if tc.query != "" {
					err = c.ShouldBindQuery(tc.request)
				} else {
					err = c.ShouldBindJSON(tc.request)
				}
				if err != nil {
					if tc.wrap {
						err = fmt.Errorf("private wrapper: %w", err)
					}
					RespondBindError(c, err)
					return
				}
				if tc.message != "" {
					t.Fatal("expected a binding error")
				}
				c.AbortWithStatus(http.StatusNoContent)
			})
			handler.POST("/", func(c *gin.Context) { t.Error("handler ran after response") })
			url := "/"
			if tc.query != "" {
				url = tc.query
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, url, strings.NewReader(tc.body)))
			if tc.message == "" {
				if recorder.Code != http.StatusNoContent {
					t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
				}
				return
			}
			assertBindingResponse(t, recorder, 400, 100001, tc.message)
		})
	}
}

func TestRespondBindErrorFallback(t *testing.T) {
	type requiredReq struct {
		Name string `json:"name" validate:"required"`
	}
	v := validator.New()
	v.RegisterTagNameFunc(bindingFieldName)
	validationErr := v.Struct(requiredReq{})
	invalidErr := validator.New().Struct(42)
	for _, tc := range []struct {
		name    string
		err     error
		status  int
		code    int
		message string
	}{
		{"nil error", nil, 500, -1, "未知错误"},
		{"invalid validator use", invalidErr, 500, -1, "未知错误"},
		{"wrapped invalid use", fmt.Errorf("private detail: %w", invalidErr), 500, -1, "未知错误"},
		{"other bind error", errors.New("private-token"), 400, 100001, "请求参数不合法"},
		{"empty validation errors", validator.ValidationErrors{}, 400, 100001, "请求参数不合法"},
		{"registered field name", validationErr, 400, 100001, "name 格式校验失败"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := gin.New()
			handler.GET("/", func(c *gin.Context) {
				RespondBindError(c, tc.err)
			})
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
			assertBindingResponse(t, recorder, tc.status, tc.code, tc.message)
		})
	}
}

func assertBindingResponse(t *testing.T, recorder *httptest.ResponseRecorder, status, code int, message string) {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"code": float64(code), "message": message, "data": nil}
	if recorder.Code != status || !reflect.DeepEqual(body, want) {
		t.Fatalf("status = %d, body = %s; want status %d, body %v", recorder.Code, recorder.Body.String(), status, want)
	}
}
