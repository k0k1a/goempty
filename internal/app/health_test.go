package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"goempty/internal/service"
)

func TestHTTPHealthEndpoints(t *testing.T) {
	handler := newHTTPHandler(&httpDependencies{
		Hello: service.NewHelloService(helloRepoStub{message: "hello"}),
	})
	for _, path := range []string{"/healthz", "/readyz"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.Header.Set("X-Request-ID", "health-request-1")
			request.Header.Set("Origin", "https://frontend.example")
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			if recorder.Body.Len() != 0 {
				t.Fatalf("expected empty body, got %q", recorder.Body.String())
			}
			if recorder.Header().Get("X-Request-ID") != "health-request-1" ||
				recorder.Header().Get("Access-Control-Allow-Origin") != "*" {
				t.Fatalf("missing middleware headers: %v", recorder.Header())
			}

			recorder = httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, nil))
			if recorder.Code != http.StatusBadRequest || recorder.Header().Get("Allow") != http.MethodGet {
				t.Fatalf("POST status = %d, Allow = %q", recorder.Code, recorder.Header().Get("Allow"))
			}
		})
	}
}
