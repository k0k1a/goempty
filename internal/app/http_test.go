package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"goempty/internal/repo/db"
	"goempty/internal/service"

	"github.com/gin-gonic/gin"
)

type helloRepoStub struct {
	message string
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
		if response.Code != http.StatusBadRequest {
			t.Errorf("GET %s status = %d, want %d", path, response.Code, http.StatusBadRequest)
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
		{http.MethodGet, "/missing", http.StatusBadRequest, 100004},
		{http.MethodPost, "/hello", http.StatusBadRequest, 100001},
		{http.MethodGet, "/panic", http.StatusBadRequest, -1},
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
			if tc.path == "/panic" && body.Message != "未知错误" {
				t.Fatalf("private panic exposed: %s", recorder.Body.String())
			}
			if tc.method == http.MethodPost && recorder.Header().Get("Allow") != http.MethodGet {
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
		{"/panic", http.StatusBadRequest},
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
		{http.MethodGet, "/missing", http.StatusBadRequest},
		{http.MethodPost, "/hello", http.StatusBadRequest},
		{http.MethodGet, "/panic-id", http.StatusBadRequest},
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
