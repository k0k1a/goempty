package hello

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHello(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		valid      bool
	}{
		{"success", `{"message":"from upstream"}`, 200, true},
		{"status error", "private upstream detail", 503, false},
		{"invalid JSON", "private invalid payload", 200, false},
		{"missing message", `{}`, 200, false},
		{"trailing JSON", `{"message":"hello"}{}`, 200, false},
		{"large response", strings.Repeat("x", maxResponseBytes+1), 200, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/api/hello" || r.Header.Get("Accept") != "application/json" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client, err := New(Config{BaseURL: server.URL + "/api/"})
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.Hello(context.Background())
			if (err == nil) != tc.valid {
				t.Fatalf("Hello() = %+v, %v", result, err)
			}
			if tc.valid && result.Message != "from upstream" {
				t.Fatalf("result = %+v", result)
			}
			if err != nil && strings.Contains(err.Error(), "private") {
				t.Fatal("upstream body leaked")
			}
		})
	}
}

func TestHelloCancellationAndTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	client, err := New(Config{BaseURL: server.URL, Timeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Hello(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled = %v", err)
	}
	if _, err := client.Hello(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout = %v", err)
	}
}

func TestNewValidation(t *testing.T) {
	for _, base := range []string{"", "/relative", "ftp://example.com", "http://", "https://user:secret@example.com", "https://example.com?token=secret", "https://example.com#fragment"} {
		if _, err := New(Config{BaseURL: base}); err == nil {
			t.Errorf("accepted invalid URL %q", base)
		}
	}
	if _, err := New(Config{BaseURL: "https://example.com", Timeout: -time.Second}); err == nil {
		t.Fatal("accepted negative timeout")
	}
}
