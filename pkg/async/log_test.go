package async

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Use a subprocess so the immutable project logger is tested without replacing
// global state or capturing output from concurrent tests.
func TestPoolLogFields(t *testing.T) {
	if os.Getenv("GOEMPTY_ASYNC_LOG_PROCESS") == "1" {
		p := testPool(t, Config{Concurrency: 3})
		ctx := context.WithValue(context.Background(), "trace_id", "async-trace")
		for name, handler := range map[string]Handler{
			"panic":  func(context.Context) error { panic("logged panic") },
			"error":  func(context.Context) error { return errors.New("logged error") },
			"normal": func(context.Context) error { return nil },
		} {
			if err := p.Submit(ctx, name, handler); err != nil {
				t.Fatal(err)
			}
		}
		return // testPool cleanup waits for all log writes before the process exits.
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPoolLogFields$")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "GOEMPTY_ASYNC_LOG_PROCESS=1", "LEVEL=info", "APP_NAME=", "ENV=")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("log process: %v\n%s", err, output)
	}
	seen := make(map[string]bool)
	for _, line := range strings.Split(string(output), "\n") {
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal(err)
		}
		if entry["trace_id"] != "async-trace" {
			t.Fatalf("missing trace: %s", line)
		}
		switch entry["msg"] {
		case "Async task panicked":
			if entry["task_name"] != "panic" || entry["panic"] != "logged panic" || entry["stack"] == "" || entry["stack"] == nil {
				t.Fatalf("panic log: %s", line)
			}
			seen["panic"] = true
		case "Async task failed":
			if entry["task_name"] != "error" || entry["error"] != "logged error" {
				t.Fatalf("error log: %s", line)
			}
			seen["error"] = true
		case "Async task completed":
			if entry["task_name"] == "normal" {
				seen["normal"] = true
			}
		}
	}
	for _, name := range []string{"panic", "error", "normal"} {
		if !seen[name] {
			t.Fatalf("missing %s log\n%s", name, output)
		}
	}
}
