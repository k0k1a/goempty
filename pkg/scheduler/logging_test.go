package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"goempty/pkg/log"

	"github.com/google/uuid"
)

type taskLogEntry struct {
	Message string `json:"msg"`
	Task    string `json:"task_name"`
	TraceID string `json:"trace_id"`
}

func taskLogEntries(t *testing.T, mode string) []taskLogEntry {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestTaskLoggingProcess$")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "GOEMPTY_TASK_LOG_TEST="+mode, "APP_NAME=", "ENV=", "LEVEL=")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("task logging process: %v\n%s", err, output)
	}
	var entries []taskLogEntry
	for _, line := range strings.Split(string(output), "\n") {
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var entry taskLogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	return entries
}

func TestTaskRunLogCorrelation(t *testing.T) {
	entries := taskLogEntries(t, "runs")
	runs := make(map[string][]taskLogEntry)
	for _, entry := range entries {
		id, err := uuid.Parse(entry.TraceID)
		if err != nil || id.Version() != 4 || id.Variant() != uuid.RFC4122 {
			t.Fatalf("invalid UUID v4: %+v", entry)
		}
		runs[entry.TraceID] = append(runs[entry.TraceID], entry)
	}
	if len(runs) != 13 {
		t.Fatalf("got %d distinct executions, want 13", len(runs))
	}
	terminals := make(map[string]int)
	for _, entries := range runs {
		if len(entries) != 3 || entries[0].Message != "Scheduled task started" || entries[1].Message != "business log" {
			t.Fatalf("incomplete execution logs: %+v", entries)
		}
		for _, entry := range []taskLogEntry{entries[0], entries[2]} {
			if entry.Task == "" || entry.Task != entries[0].Task {
				t.Fatalf("task context crossed execution boundaries: %+v", entries)
			}
		}
		if entries[1].Task != "" {
			t.Fatalf("business log unexpectedly received task_name: %+v", entries[1])
		}
		terminals[entries[2].Message]++
	}
	if terminals["Scheduled task completed"] != 11 || terminals["Scheduled task failed"] != 1 || terminals["Scheduled task panicked"] != 1 {
		t.Fatalf("unexpected execution results: %v", terminals)
	}
}

func TestTaskTraceIDGenerationFailure(t *testing.T) {
	entries := taskLogEntries(t, "random-failure")
	if len(entries) != 1 || entries[0].Message != "Generate task trace ID failed" || entries[0].Task != "failure" || entries[0].TraceID != "" {
		t.Fatalf("unexpected generation failure logs: %+v", entries)
	}
}

type failedRandomReader struct{}

func (failedRandomReader) Read([]byte) (int, error) {
	return 0, errors.New("random source unavailable")
}

func TestTaskLoggingProcess(t *testing.T) {
	mode := os.Getenv("GOEMPTY_TASK_LOG_TEST")
	if mode == "" {
		t.Skip("subprocess helper")
	}
	s := newTestScheduler(t)
	s.runCtx = context.Background()
	switch mode {
	case "runs":
		for _, result := range []string{"success", "error", "panic"} {
			s.execute(Task{Name: "same-task", Handler: func(ctx context.Context) error {
				log.Info(ctx, "business log")
				if result == "panic" {
					panic("test panic")
				}
				if result == "error" {
					return errors.New("test failure")
				}
				return nil
			}})
		}
		var wg sync.WaitGroup
		for _, name := range []string{"first", "second"} {
			wg.Go(func() {
				for range 5 {
					s.execute(Task{Name: name, Handler: func(ctx context.Context) error {
						log.Info(ctx, "business log")
						return nil
					}})
				}
			})
		}
		wg.Wait()
	case "random-failure":
		// Isolated subprocess: never change the UUID source in the main test process.
		uuid.SetRand(failedRandomReader{})
		defer uuid.SetRand(nil)
		s.execute(Task{Name: "failure", Handler: func(context.Context) error {
			t.Fatal("handler called without a trace ID")
			return nil
		}})
	default:
		t.Fatalf("unknown mode %q", mode)
	}
	if err := log.Shutdown(); err != nil {
		t.Fatal(err)
	}
}
