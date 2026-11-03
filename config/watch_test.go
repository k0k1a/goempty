package config

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Use a subprocess because Viper's watcher has process lifetime and no Close API.
func TestWatchConfigReload(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestConfigWatchProcess$")
	cmd.Env = append(os.Environ(), "GOEMPTY_CONFIG_WATCH_TEST=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stdin.Close()
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	lines := make(chan string, 32)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()
	waitFor := func(want string) {
		t.Helper()
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		for {
			select {
			case line, ok := <-lines:
				if !ok {
					t.Fatalf("watch process exited before %q", want)
				}
				if strings.Contains(line, want) {
					return
				}
			case <-timer.C:
				t.Fatalf("timeout waiting for %q", want)
			}
		}
	}
	send := func(command string) {
		t.Helper()
		if _, err := fmt.Fprintln(stdin, command); err != nil {
			t.Fatal(err)
		}
	}
	waitFor("WATCH_READY")
	send("valid")
	waitFor("Configuration reloaded")
	send("invalid")
	waitFor("Configuration reload failed")
	send("valid")
	waitFor("Configuration reloaded")
	send("quit")
	waitFor("WATCH_DONE")
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestConfigWatchProcess(t *testing.T) {
	if os.Getenv("GOEMPTY_CONFIG_WATCH_TEST") != "1" {
		t.Skip("subprocess helper")
	}
	setupConfig(t, "app_dev.yaml", "{}\n")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	var readers sync.WaitGroup
	for range 4 {
		readers.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					_ = Get()
				}
			}
		})
	}
	defer func() {
		close(stop)
		readers.Wait()
	}()
	fmt.Println("WATCH_READY")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		content := "{}\n"
		switch scanner.Text() {
		case "invalid":
			content = "unknown: true\n"
		case "quit":
			fmt.Println("WATCH_DONE")
			return
		}
		if err := os.WriteFile(filepath.Join("configs", "app_dev.yaml"), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}
