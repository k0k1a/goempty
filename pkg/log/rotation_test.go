package log_test

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	applog "goempty/pkg/log"
)

func TestSizeRotationCompressionAndRetention(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	expired := filepath.Join(dir, "app-2000-01-01T00-00-00.000.log")
	if err := os.WriteFile(expired, []byte("expired"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(path)
	cfg.MaxSize = 1
	logger, err := applog.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := applog.Close(logger); err != nil {
			t.Error(err)
		}
	})
	// Each entry fits in 1 MB; the pair forces automatic size rotation.
	payload := zap.String("payload", strings.Repeat("x", 600*1024))
	logger.Info("before rotation", payload)
	logger.Info("after rotation", payload)
	if err := applog.Close(logger); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	assertMessage(t, data, "after rotation")
	// Lumberjack performs gzip and age cleanup asynchronously.
	deadline := time.Now().Add(5 * time.Second)
	for {
		files, err := filepath.Glob(filepath.Join(dir, "app-*.log.gz"))
		if err != nil {
			t.Fatal(err)
		}
		_, ageErr := os.Stat(expired)
		if len(files) == 1 && os.IsNotExist(ageErr) {
			data, err = readGzip(files[0])
			if err == nil {
				assertMessage(t, data, "before rotation")
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("compression/cleanup did not complete: files=%v, expired=%v, gzip=%v", files, ageErr, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func readGzip(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func assertMessage(t *testing.T, data []byte, want string) {
	t.Helper()
	var entry struct {
		Message string `json:"msg"`
	}
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("expected one complete JSON entry: %v", err)
	}
	if entry.Message != want {
		t.Fatalf("message = %q, want %q", entry.Message, want)
	}
}

func TestCloseSharedFileOutputs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	cfg := testConfig(path)
	cfg.ErrorOutputPaths = []string{path}
	logger, err := applog.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	child := logger.With(zap.String("request_id", "test")).Named("app")
	child.Info("written")
	if err := applog.Close(child); err != nil {
		t.Fatal(err)
	}
	if err := applog.Close(logger); err != nil {
		t.Fatal(err)
	}
	// On Windows this fails if either output still has the file open.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if applog.Close(nil) == nil || applog.Close(zap.NewNop()) == nil {
		t.Fatal("Close must reject nil and unmanaged loggers")
	}
}

func TestFailedInitializationClosesFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	blocker := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(path)
	cfg.ErrorOutputPaths = []string{filepath.Join(blocker, "error.log")}
	logger, err := applog.New(cfg)
	if err == nil || logger != nil {
		t.Fatalf("New = %v, %v; want failure", logger, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("partially initialized output remains open: %v", err)
	}
}

func TestRotationWithoutCompressionOrAgeLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	expired := filepath.Join(dir, "app-2000-01-01T00-00-00.000.log")
	if err := os.WriteFile(expired, []byte("retained"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(path)
	cfg.MaxSize, cfg.MaxAge, cfg.MaxBackups, cfg.Compress = 1, 0, 0, false
	logger, err := applog.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	payload := zap.String("payload", strings.Repeat("x", 600*1024))
	logger.Info("before rotation", payload)
	logger.Info("after rotation", payload)
	if err := applog.Close(logger); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(filepath.Join(dir, "app-*.log"))
	if err != nil || len(backups) != 2 {
		t.Fatalf("uncompressed backups = %v, %v; want old and newly rotated files", backups, err)
	}
	for _, backup := range backups {
		data, err := os.ReadFile(backup)
		if err != nil {
			t.Fatal(err)
		}
		if backup == expired {
			if string(data) != "retained" {
				t.Fatalf("age-unlimited backup changed: %q", data)
			}
		} else {
			assertMessage(t, data, "before rotation")
		}
	}
}
