package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goempty/internal/service"

	"github.com/gin-gonic/gin"
)

func TestHTTPCORS(t *testing.T) {
	handler := newHTTPHandler(&httpDependencies{
		Hello: service.NewHelloService(helloRepoStub{message: "hello"}),
	})
	handler.GET("/cors-panic", func(c *gin.Context) { panic("test CORS recovery") })
	for _, tc := range []struct {
		name   string
		method string
		path   string
		origin string
		status int
	}{
		{"ordinary request", http.MethodGet, "/hello", "", http.StatusOK},
		{"cross origin", http.MethodGet, "/hello", "https://frontend.example", http.StatusOK},
		{"another origin", http.MethodGet, "/hello", "https://other.example", http.StatusOK},
		{"not found", http.MethodGet, "/missing", "https://frontend.example", http.StatusBadRequest},
		{"method not allowed", http.MethodPost, "/hello", "https://frontend.example", http.StatusBadRequest},
		{"panic", http.MethodGet, "/cors-panic", "https://frontend.example", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, nil)
			request.Header.Set("Origin", tc.origin)
			request.Header.Set("X-Request-ID", "cors-request-1")
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != tc.status {
				t.Fatalf("status = %d, want %d", recorder.Code, tc.status)
			}
			wantOrigin := ""
			if tc.origin != "" {
				wantOrigin = "*"
				if !headerContains(recorder.Header(), "Access-Control-Expose-Headers", "X-Request-ID") {
					t.Error("X-Request-ID is not exposed to the browser")
				}
			}
			if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != wantOrigin {
				t.Errorf("allow origin = %q, want %q", got, wantOrigin)
			}
			if recorder.Header().Get("Access-Control-Allow-Credentials") != "" {
				t.Error("cross-origin credentials enabled")
			}
			if recorder.Header().Get("X-Request-ID") != "cors-request-1" {
				t.Error("request ID lost")
			}
		})
	}
}

func TestHTTPCORSPreflight(t *testing.T) {
	handler := newHTTPHandler(&httpDependencies{
		Hello: service.NewHelloService(helloRepoStub{message: "hello"}),
	})
	request := httptest.NewRequest(http.MethodOptions, "/hello", nil)
	request.Header.Set("Origin", "https://frontend.example")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	request.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type, X-Request-ID")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent || recorder.Body.Len() != 0 {
		t.Fatalf("preflight status = %d, body = %q", recorder.Code, recorder.Body.String())
	}
	headers := recorder.Header()
	if headers.Get("Access-Control-Allow-Origin") != "*" || headers.Get("X-Request-ID") == "" {
		t.Fatalf("missing preflight headers: %v", headers)
	}
	for _, name := range []string{"Authorization", "Content-Type", "X-Request-ID"} {
		if !headerContains(headers, "Access-Control-Allow-Headers", name) {
			t.Errorf("preflight does not allow header %s", name)
		}
	}
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"} {
		if !headerContains(headers, "Access-Control-Allow-Methods", method) {
			t.Errorf("preflight does not allow method %s", method)
		}
	}
	if headers.Get("Access-Control-Allow-Credentials") != "" {
		t.Error("preflight allows credentials")
	}
}

func headerContains(headers http.Header, key, want string) bool {
	for _, value := range headers.Values(key) {
		for _, token := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(token), want) {
				return true
			}
		}
	}
	return false
}
