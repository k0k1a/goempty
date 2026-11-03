package log_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	applog "goempty/pkg/log"
)

func TestJSONOutput(t *testing.T) {
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "app.log"))
	writeLogs(t, testConfig(path))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d entries, want 2: %s", len(lines), data)
	}
	for i, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal(err)
		}
		if entry["logger"] != "app" || entry["request_id"] != "request-1" {
			t.Fatalf("missing inherited fields: %v", entry)
		}
		if caller, _ := entry["caller"].(string); !strings.Contains(caller, "log_test.go:") {
			t.Fatalf("incorrect caller: %v", entry)
		}
		if stamp, _ := entry["time"].(string); stamp == "" {
			t.Fatalf("missing time: %v", entry)
		} else if _, err := time.Parse("2006-01-02T15:04:05.000Z0700", stamp); err != nil {
			t.Fatalf("invalid ISO 8601 time: %v", err)
		}
		if i == 0 {
			if entry["level"] != "info" || entry["msg"] != "started" || entry["stacktrace"] != nil {
				t.Fatalf("unexpected info entry: %v", entry)
			}
		} else if entry["level"] != "error" || entry["msg"] != "failed" || entry["code"] != float64(42) || entry["stacktrace"] == nil {
			t.Fatalf("unexpected error entry: %v", entry)
		}
	}
}

func TestConsoleOutputAndAppend(t *testing.T) {
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "app.log"))
	if err := os.WriteFile(path, []byte("existing\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(path)
	cfg.Level, cfg.Encoding = "debug", "console"
	writeLogs(t, cfg)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "existing\n") || !strings.Contains(string(data), "visible") || !strings.Contains(string(data), `"key": "value"`) || json.Valid(data) {
		t.Fatalf("unexpected console output: %s", data)
	}
}

func writeLogs(t *testing.T, cfg applog.Config) {
	t.Helper()
	logger, err := applog.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Encoding == "console" {
		logger.Debug("visible", zap.String("key", "value"))
	} else {
		logger.Debug("hidden")
		child := logger.Named("app").With(zap.String("request_id", "request-1"))
		child.Info("started")
		child.Error("failed", zap.Int("code", 42))
	}
	if err := applog.Close(logger); err != nil {
		t.Fatal(err)
	}
}

func TestConfig(t *testing.T) {
	cfg := testConfig("stdout")
	cfg.Level, cfg.MaxAge, cfg.Compress = "error", 0, false
	logger, err := applog.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if logger.Level() != zapcore.ErrorLevel {
		t.Fatalf("configured level = %v", logger.Level())
	}
	if err := applog.Close(logger); err != nil {
		t.Fatal(err)
	}
	for name, invalidate := range map[string]func(*applog.Config){
		"empty config":         func(c *applog.Config) { *c = applog.Config{} },
		"missing level":        func(c *applog.Config) { c.Level = "" },
		"invalid level":        func(c *applog.Config) { c.Level = "invalid" },
		"missing encoding":     func(c *applog.Config) { c.Encoding = "" },
		"invalid encoding":     func(c *applog.Config) { c.Encoding = "xml" },
		"missing output":       func(c *applog.Config) { c.OutputPaths = nil },
		"missing error output": func(c *applog.Config) { c.ErrorOutputPaths = nil },
		"blank output":         func(c *applog.Config) { c.OutputPaths = []string{""} },
		"blank error output":   func(c *applog.Config) { c.ErrorOutputPaths = []string{" "} },
		"zero size":            func(c *applog.Config) { c.MaxSize = 0 },
		"negative size":        func(c *applog.Config) { c.MaxSize = -1 },
		"negative age":         func(c *applog.Config) { c.MaxAge = -1 },
		"negative backups":     func(c *applog.Config) { c.MaxBackups = -1 },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := testConfig("stdout")
			invalidate(&cfg)
			if cfg.Validate() == nil {
				t.Fatalf("Validate(%+v) succeeded", cfg)
			}
			if logger, err := applog.New(cfg); err == nil || logger != nil {
				t.Fatalf("New(%+v) = %v, %v", cfg, logger, err)
			}
		})
	}
}

func TestUnavailableOutput(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("block"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.ToSlash(filepath.Join(blocker, "app.log"))
	for _, errorOutput := range []bool{false, true} {
		cfg := testConfig("stdout")
		if errorOutput {
			cfg.ErrorOutputPaths = []string{path}
		} else {
			cfg.OutputPaths = []string{path}
		}
		if logger, err := applog.New(cfg); err == nil || logger != nil {
			t.Fatalf("New(%+v) = %v, %v", cfg, logger, err)
		}
	}
}

func TestMultipleFileOutputs(t *testing.T) {
	dir := t.TempDir()
	paths := []string{filepath.ToSlash(filepath.Join(dir, "app.log")), filepath.ToSlash(filepath.Join(dir, "other.log"))}
	writeLogs(t, testConfig(paths...))
	first, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(paths[1])
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) || len(first) == 0 {
		t.Fatal("both fixed output files must receive the same entries")
	}
}

func testConfig(paths ...string) applog.Config {
	return applog.Config{
		Level: "info", Encoding: "json", OutputPaths: paths,
		ErrorOutputPaths: []string{"stderr"}, MaxSize: 100,
		MaxAge: 90, MaxBackups: 0, Compress: true,
	}
}
