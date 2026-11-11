package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goempty/internal/controller/response"
	"goempty/internal/repo/db"
	"goempty/internal/service"
	"goempty/pkg/httpserver"

	"github.com/gin-gonic/gin"
)

type helloRepoStub struct {
	message string
}

func TestHTTPBindingValidation(t *testing.T) {
	deps := &httpDependencies{Hello: service.NewHelloService(helloRepoStub{message: "hello"})}
	// Constructing the server must configure validation before requests arrive.
	// New does not open a listener, so this test needs no external services.
	if _, err := newHTTPServer(httpserver.Config{Addr: ":0"}, deps); err != nil {
		t.Fatal(err)
	}
	handler := newHTTPHandler(deps)
	handler.POST("/binding-test", func(c *gin.Context) {
		var req struct {
			InternalName string `json:"name" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.RespondBindError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/binding-test", strings.NewReader(`{}`)))
	var body response.Response[any]
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusBadRequest || body.Code != 100001 ||
		body.Message != "name 格式校验失败" || body.Data != nil {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func (r helloRepoStub) Hello() string {
	return r.message
}

func TestHTTPHelloRoute(t *testing.T) {
	// A distinct greeting verifies that the response comes through the injected repository.
	handler := newHTTPHandler(&httpDependencies{
		Hello: service.NewHelloService(helloRepoStub{message: "from repository"}),
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/hello", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	var body struct {
		Code int `json:"code"`
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != 0 || body.Data.Message != "from repository" {
		t.Fatalf("body = %+v, want code 0 and greeting from repository", body)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["code"]) != "0" {
		t.Fatalf("code must be present as JSON number 0: %s", response.Body.String())
	}

	for _, path := range []string{"/", "/missing"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want %d", path, response.Code, http.StatusNotFound)
		}
	}
}

func TestHTTPFrameworkErrors(t *testing.T) {
	handler := newHTTPHandler(&httpDependencies{
		Hello: service.NewHelloService(helloRepoStub{message: "hello"}),
	})
	handler.GET("/panic", func(c *gin.Context) { panic("private panic detail") })
	handler.GET("/written-panic", func(c *gin.Context) {
		c.String(http.StatusOK, "already sent")
		panic("private panic detail")
	})
	for _, tc := range []struct {
		method string
		path   string
		status int
		code   int
	}{
		{http.MethodGet, "/missing", http.StatusNotFound, 100004},
		{http.MethodPost, "/hello", http.StatusMethodNotAllowed, 100001},
		{http.MethodGet, "/panic", http.StatusInternalServerError, -1},
	} {
		t.Run(tc.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, nil))
			var body struct {
				Code    int             `json:"code"`
				Message string          `json:"message"`
				Data    json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != tc.status || body.Code != tc.code || string(body.Data) != "null" {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			if tc.status == http.StatusInternalServerError && body.Message != "未知错误" {
				t.Fatalf("private panic exposed: %s", recorder.Body.String())
			}
			if tc.status == http.StatusMethodNotAllowed && recorder.Header().Get("Allow") != http.MethodGet {
				t.Fatalf("Allow = %q, want GET", recorder.Header().Get("Allow"))
			}
		})
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/written-panic", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "already sent" {
		t.Fatalf("response corrupted after panic: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestHTTPRecovery(t *testing.T) {
	handler := newHTTPHandler(&httpDependencies{
		Hello: service.NewHelloService(db.NewHelloRepo()),
	})
	handler.GET("/panic", func(c *gin.Context) {
		panic("test recovery")
	})
	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/panic", http.StatusInternalServerError},
		{"/hello", http.StatusOK},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if response.Code != tc.status {
			t.Errorf("GET %s status = %d, want %d", tc.path, response.Code, tc.status)
		}
	}
}

func TestHTTPRequestID(t *testing.T) {
	handler := newHTTPHandler(&httpDependencies{
		Hello: service.NewHelloService(helloRepoStub{message: "hello"}),
	})
	handler.GET("/panic-id", func(c *gin.Context) { panic("test request ID") })
	for _, tc := range []struct {
		method string
		path   string
		status int
	}{
		{http.MethodGet, "/hello", http.StatusOK},
		{http.MethodGet, "/missing", http.StatusNotFound},
		{http.MethodPost, "/hello", http.StatusMethodNotAllowed},
		{http.MethodGet, "/panic-id", http.StatusInternalServerError},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, nil)
			request.Header.Set("X-Request-ID", "app-request-1")
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != tc.status || recorder.Header().Get("X-Request-ID") != "app-request-1" {
				t.Fatalf("status = %d, X-Request-ID = %q", recorder.Code, recorder.Header().Get("X-Request-ID"))
			}
		})
	}
}
