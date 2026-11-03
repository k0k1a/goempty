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
}

func defaultLogEntries(t *testing.T, mode string) []globalEntry {
	t.Helper()
	// Each test gets a fresh default logger and shuts it down only after writing.
	cmd := exec.Command(os.Args[0], "-test.run=^TestDefaultLoggerProcess$")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "GOEMPTY_DEFAULT_LOG_TEST="+mode, "APP_NAME=", "ENV=", "LEVEL=")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("default logger process: %v\n%s", err, output)
	}
	var entries []globalEntry
	for _, line := range strings.Split(string(output), "\n") {
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
					t.Fatalf("expected startup failure for invalid level: %v\n%s", err, output)
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
				ctx := applog.WithRequestID(context.Background(), fmt.Sprintf("request-%d", i))
				for j := range 100 {
					applog.Info(ctx, fmt.Sprintf("%d:%d", i, j))
				}
			})
		}
		group.Wait()
	case "request-id":
		parent := context.WithValue(context.Background(), "request_id", "unrelated-key")
		ctx := applog.WithRequestID(parent, "request-1")
		for _, write := range []func(context.Context, string, ...zap.Field){applog.Info, applog.Warn, applog.Error} {
			_, _, line, _ := runtime.Caller(0)
			write(ctx, "with ID", zap.Int("expected_line", line+1))
		}
		applog.Info(applog.WithRequestID(ctx, "request-2"), "overridden ID")
		applog.Info(ctx, "parent ID unchanged")
		applog.Info(parent, "without ID")
		applog.Info(applog.WithRequestID(ctx, ""), "empty ID")
		applog.Info(nil, "nil context")
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
