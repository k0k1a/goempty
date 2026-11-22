package log_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap"
	applog "goempty/pkg/log"
)

type globalEntry struct {
	Level        string  `json:"level"`
	Message      string  `json:"msg"`
	Caller       string  `json:"caller"`
	ExpectedLine int     `json:"expected_line"`
	Time         string  `json:"time"`
	RequestID    *string `json:"request_id"`
	Task         string  `json:"task_name"`
	TraceID      string  `json:"trace_id"`
}

func runDefaultLoggerProcess(t *testing.T, dir, mode string) (string, error) {
	t.Helper()
	// Each test gets a fresh default logger initialized on first use.
	cmd := exec.Command(os.Args[0], "-test.run=^TestDefaultLoggerProcess$")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOEMPTY_DEFAULT_LOG_TEST="+mode, "APP_NAME=", "ENV=", "LEVEL=")
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func defaultLogEntries(t *testing.T, mode string) []globalEntry {
	t.Helper()
	output, err := runDefaultLoggerProcess(t, t.TempDir(), mode)
	if err != nil {
		t.Fatalf("default logger process: %v\n%s", err, output)
	}
	var entries []globalEntry
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var entry globalEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	return entries
}

func TestDefaultLoggerEnvironment(t *testing.T) {
	for _, tc := range []struct {
		name, appName, environment, level, filename string
		wantDebug, wantFailure                      bool
	}{
		{name: "defaults", filename: "app_dev.log"},
		{name: "custom", appName: "order", environment: "prod", level: "debug", filename: "order_prod.log", wantDebug: true},
		{name: "invalid level", level: "invalid", wantFailure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cmd := exec.Command(os.Args[0], "-test.run=^TestDefaultLoggerProcess$")
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "GOEMPTY_DEFAULT_LOG_TEST=levels",
				"APP_NAME="+tc.appName, "ENV="+tc.environment, "LEVEL="+tc.level)
			output, err := cmd.CombinedOutput()
			if tc.wantFailure {
				if err == nil || !strings.Contains(string(output), "log: invalid level") {
					t.Fatalf("expected first-log failure for invalid level: %v\n%s", err, output)
				}
				return
			}
			if err != nil {
				t.Fatalf("default logger process: %v\n%s", err, output)
			}
			data, err := os.ReadFile(filepath.Join(dir, "logs", tc.filename))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), `"level":"debug"`) != tc.wantDebug {
				t.Fatalf("unexpected debug filtering: %s", data)
			}
			for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
				if !json.Valid([]byte(line)) || !strings.Contains(string(output), line) {
					t.Fatalf("file entry missing from standard output or invalid JSON: %s", line)
				}
			}
			if !strings.Contains(string(data), `"level":"info"`) {
				t.Fatalf("missing info entry: %s", data)
			}
		})
	}
}

func TestDefaultLoggerWithoutInitialization(t *testing.T) {
	entries := defaultLogEntries(t, "levels")
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want info/warn/error", len(entries))
	}
	for i, level := range []string{"info", "warn", "error"} {
		entry := entries[i]
		if entry.Level != level || entry.Message != "ready without initialization" || entry.Time == "" {
			t.Fatalf("unexpected default entry: %+v", entry)
		}
		if !strings.HasSuffix(entry.Caller, fmt.Sprintf("global_test.go:%d", entry.ExpectedLine)) {
			t.Fatalf("default caller = %s, expected line %d", entry.Caller, entry.ExpectedLine)
		}
	}
}

func TestDefaultLoggerLazyInitialization(t *testing.T) {
	dir := t.TempDir()
	output, err := runDefaultLoggerProcess(t, dir, "shutdown")
	if err != nil {
		t.Fatalf("default logger process: %v\n%s", err, output)
	}
	if _, err := os.Stat(filepath.Join(dir, "logs")); !os.IsNotExist(err) {
		t.Fatalf("logs directory created without logging: %v", err)
	}
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "{") {
			t.Fatalf("unexpected log entry: %s", line)
		}
	}
}

func TestDefaultLoggerShutdownStopsWriting(t *testing.T) {
	dir := t.TempDir()
	output, err := runDefaultLoggerProcess(t, dir, "write-after-shutdown")
	if err != nil {
		t.Fatalf("default logger process: %v\n%s", err, output)
	}
	data, err := os.ReadFile(filepath.Join(dir, "logs", "app_dev.log"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], `"msg":"before shutdown"`) {
		t.Fatalf("log file changed after shutdown: %s", data)
	}
	if got := countJSONLines(output); got != 1 {
		t.Fatalf("got %d stdout entries after shutdown, want 1: %s", got, output)
	}
}

func TestDefaultLoggerZapAccessor(t *testing.T) {
	entries := defaultLogEntries(t, "zap")
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want one direct entry", len(entries))
	}
	entry := entries[0]
	if entry.Level != "info" || entry.Message != "direct call" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if !strings.HasSuffix(entry.Caller, fmt.Sprintf("global_test.go:%d", entry.ExpectedLine)) {
		t.Fatalf("zap caller = %s, expected line %d", entry.Caller, entry.ExpectedLine)
	}
}

func countJSONLines(output string) int {
	count := 0
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "{") {
			count++
		}
	}
	return count
}

func TestConcurrentDefaultLogging(t *testing.T) {
	entries := defaultLogEntries(t, "concurrent")
	seen := make(map[string]bool)
	for _, entry := range entries {
		if entry.Level != "info" {
			t.Fatalf("unexpected concurrent entry: %+v", entry)
		}
		parts := strings.SplitN(entry.Message, ":", 2)
		if entry.RequestID == nil || *entry.RequestID != "request-"+parts[0] {
			t.Fatalf("request ID crossed request boundaries: %+v", entry)
		}
		seen[entry.Message] = true
	}
	if len(entries) != 1000 || len(seen) != 1000 {
		t.Fatalf("got %d entries, %d unique, want 1000", len(entries), len(seen))
	}
}

func TestDefaultLoggerProcess(t *testing.T) {
	switch os.Getenv("GOEMPTY_DEFAULT_LOG_TEST") {
	case "levels":
		applog.Debug(context.Background(), "hidden by default")
		for _, write := range []func(context.Context, string, ...zap.Field){applog.Info, applog.Warn, applog.Error} {
			_, _, line, _ := runtime.Caller(0)
			write(context.Background(), "ready without initialization", zap.Int("expected_line", line+1))
		}
	case "concurrent":
		var group sync.WaitGroup
		for i := range 10 {
			group.Go(func() {
				ctx := context.WithValue(context.Background(), "request_id", fmt.Sprintf("request-%d", i))
				for j := range 100 {
					applog.Info(ctx, fmt.Sprintf("%d:%d", i, j))
				}
			})
		}
		group.Wait()
	case "shutdown":
		// Never logs; Shutdown must not create outputs.
		if err := applog.Shutdown(); err != nil {
			t.Fatal(err)
		}
	case "write-after-shutdown":
		applog.Info(context.Background(), "before shutdown")
		if err := applog.Shutdown(); err != nil {
			t.Fatal(err)
		}
		applog.Info(context.Background(), "after shutdown")
		applog.Error(context.Background(), "after shutdown")
	case "zap":
		_, _, line, _ := runtime.Caller(0)
		applog.Zap().Info("direct call", zap.Int("expected_line", line+1))
		if err := applog.Shutdown(); err != nil {
			t.Fatal(err)
		}
		applog.Zap().Info("after shutdown")
	case "request-id":
		parent := context.Background()
		ctx := context.WithValue(parent, "request_id", "request-1")
		for _, write := range []func(context.Context, string, ...zap.Field){applog.Info, applog.Warn, applog.Error} {
			_, _, line, _ := runtime.Caller(0)
			write(ctx, "with ID", zap.Int("expected_line", line+1))
		}
		applog.Info(context.WithValue(ctx, "request_id", "request-2"), "overridden ID")
		applog.Info(ctx, "parent ID unchanged")
		applog.Info(parent, "without ID")
		applog.Info(context.WithValue(ctx, "request_id", ""), "empty ID")
		applog.Info(nil, "nil context")
	case "trace-id":
		parent := context.WithValue(context.Background(), "request_id", "request-1")
		ctx := context.WithValue(context.WithValue(parent, "task_name", "cleanup"), "trace_id", "trace-1")
		for _, write := range []func(context.Context, string, ...zap.Field){applog.Info, applog.Warn, applog.Error} {
			write(ctx, "task log")
		}
		applog.Info(context.WithValue(ctx, "trace_id", "trace-2"), "child")
		applog.Info(ctx, "parent unchanged")
		applog.Info(parent, "without task")
		applog.Info(context.WithValue(ctx, "trace_id", ""), "empty task")
		applog.Info(context.WithValue(ctx, "trace_id", 123), "non-string trace ID")
	default:
		t.Skip("subprocess helper")
	}
	if err := applog.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if err := applog.Shutdown(); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultLoggerTraceID(t *testing.T) {
	entries := defaultLogEntries(t, "trace-id")
	wantIDs := []string{"trace-1", "trace-1", "trace-1", "trace-2", "trace-1", "", "", ""}
	if len(entries) != len(wantIDs) {
		t.Fatalf("got %d entries, want %d", len(entries), len(wantIDs))
	}
	for i, entry := range entries {
		if entry.Task != "" || entry.TraceID != wantIDs[i] || entry.RequestID == nil || *entry.RequestID != "request-1" {
			t.Fatalf("entry %d: unexpected context fields: %+v", i, entry)
		}
	}
}

func TestDefaultLoggerRequestID(t *testing.T) {
	entries := defaultLogEntries(t, "request-id")
	wantIDs := []string{"request-1", "request-1", "request-1", "request-2", "request-1", "", "", ""}
	if len(entries) != len(wantIDs) {
		t.Fatalf("got %d entries, want %d", len(entries), len(wantIDs))
	}
	for i, entry := range entries {
		if wantIDs[i] == "" {
			if entry.RequestID != nil {
				t.Fatalf("unexpected request ID: %+v", entry)
			}
		} else if entry.RequestID == nil || *entry.RequestID != wantIDs[i] {
			t.Fatalf("entry %d: got %+v, want request ID %q", i, entry, wantIDs[i])
		}
		if entry.ExpectedLine != 0 && !strings.HasSuffix(entry.Caller, fmt.Sprintf("global_test.go:%d", entry.ExpectedLine)) {
			t.Fatalf("incorrect caller with request ID: %+v", entry)
		}
	}
}
