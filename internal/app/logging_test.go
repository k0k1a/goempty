package app

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"goempty/internal/service"

	"github.com/gin-gonic/gin"
)

func TestHTTPAccessAndPanicLogs(t *testing.T) {
	if os.Getenv("GOEMPTY_HTTP_LOG_TEST") == "1" {
		handler := newHTTPHandler(&httpDependencies{
			Hello: service.NewHelloService(helloRepoStub{message: "hello"}),
		})
		handler.GET("/users/:id", func(c *gin.Context) { c.Status(http.StatusOK) })
		handler.GET("/panic", loggingPanicHandler)
		handler.GET("/written-panic", func(c *gin.Context) {
			c.String(http.StatusOK, "already sent")
			loggingPanicHandler(c)
		})
		for _, tc := range []struct {
			id, method, path string
		}{
			{"success", http.MethodGet, "/users/private-user-id"},
			{"missing", http.MethodGet, "/private-missing-path"},
			{"method", http.MethodPost, "/hello"},
			{"panic", http.MethodGet, "/panic"},
			{"written", http.MethodGet, "/written-panic"},
			{"preflight", http.MethodOptions, "/hello"},
		} {
			request := httptest.NewRequest(tc.method, tc.path+"?token=private-query-token", strings.NewReader("private-body-value"))
			request.Header.Set("X-Request-ID", tc.id)
			request.Header.Set("Authorization", "Bearer private-authorization-token")
			request.Header.Set("Cookie", "session=private-cookie-value")
			if tc.method == http.MethodOptions {
				request.Header.Set("Origin", "https://frontend.example")
				request.Header.Set("Access-Control-Request-Method", http.MethodGet)
			}
			handler.ServeHTTP(httptest.NewRecorder(), request)
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestHTTPAccessAndPanicLogs$")
	cmd.Env = append(os.Environ(), "GOEMPTY_HTTP_LOG_TEST=1", "GIN_MODE=release")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("log subprocess: %v\n%s", err, output)
	}
	for _, secret := range []string{"private-user-id", "private-missing-path", "private-query-token", "private-body-value", "private-authorization-token", "private-cookie-value"} {
		if bytes.Contains(output, []byte(secret)) {
			t.Fatalf("logs contain sensitive request value %q", secret)
		}
	}
	want := map[string]struct {
		method, route string
		status        int
	}{
		"success":   {http.MethodGet, "/users/:id", http.StatusOK},
		"missing":   {http.MethodGet, "", http.StatusNotFound},
		"method":    {http.MethodPost, "", http.StatusMethodNotAllowed},
		"panic":     {http.MethodGet, "/panic", http.StatusInternalServerError},
		"written":   {http.MethodGet, "/written-panic", http.StatusOK},
		"preflight": {http.MethodOptions, "", http.StatusNoContent},
	}
	panicCount := 0
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		if !bytes.HasPrefix(scanner.Bytes(), []byte("{")) {
			continue
		}
		var entry struct {
			Message    string `json:"msg"`
			RequestID  string `json:"request_id"`
			Method     string `json:"method"`
			Route      string `json:"route"`
			Status     int    `json:"status"`
			DurationUS *int64 `json:"duration_us"`
			PanicStack string `json:"stack"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatal(err)
		}
		if entry.Message == "HTTP request completed" {
			expected, ok := want[entry.RequestID]
			if !ok || entry.Method != expected.method || entry.Route != expected.route || entry.Status != expected.status || entry.DurationUS == nil || *entry.DurationUS < 0 {
				t.Fatalf("unexpected access log: %s", scanner.Text())
			}
			delete(want, entry.RequestID)
		} else {
			panicCount++
			if (entry.RequestID != "panic" && entry.RequestID != "written") ||
				!strings.Contains(entry.PanicStack, "loggingPanicHandler") ||
				!strings.Contains(entry.PanicStack, "logging_test.go:") {
				t.Fatalf("panic log lacks original call site or request ID: %s", scanner.Text())
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(want) != 0 || panicCount != 2 {
		t.Fatalf("missing access logs = %v, panic logs = %d", want, panicCount)
	}
}

func loggingPanicHandler(*gin.Context) { panic("test panic") }
