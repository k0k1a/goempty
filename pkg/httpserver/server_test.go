package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewValidation(t *testing.T) {
	handler := http.NotFoundHandler()
	for _, cfg := range []Config{
		{}, {Addr: "localhost"}, {Addr: "localhost:65536"},
		{Addr: ":8080", ReadHeaderTimeout: -time.Second},
		{Addr: ":8080", ReadTimeout: -time.Second},
		{Addr: ":8080", WriteTimeout: -time.Second},
		{Addr: ":8080", IdleTimeout: -time.Second},
	} {
		if _, err := New(cfg, handler); err == nil {
			t.Errorf("New(%+v) succeeded", cfg)
		}
	}
	if _, err := New(Config{Addr: ":8080"}, nil); err == nil {
		t.Fatal("New accepted nil handler")
	}
}

func TestSuppliedHandler(t *testing.T) {
	s, err := New(Config{Addr: ":0"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Handler", r.URL.Path)
		w.WriteHeader(http.StatusAccepted)
	}))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/test", nil))
	if response.Code != http.StatusAccepted || response.Header().Get("X-Handler") != "/test" {
		t.Fatalf("supplied handler response = %d, %q", response.Code, response.Header().Get("X-Handler"))
	}
}
