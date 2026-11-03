package app

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"goempty/config"
	"goempty/internal/repo/db"
	"goempty/internal/service"
	"goempty/pkg/httpserver"
)

func TestAppValidation(t *testing.T) {
	for _, cfg := range []config.Config{
		{HTTP: httpserver.Config{Addr: "invalid"}, ShutdownTimeout: time.Second},
		{HTTP: httpserver.Config{Addr: "127.0.0.1:0"}},
		{HTTP: httpserver.Config{Addr: "127.0.0.1:0"}, ShutdownTimeout: -time.Second},
	} {
		if _, err := newHTTPTestApp(cfg); err == nil {
			t.Fatalf("newHTTPTestApp(%+v) succeeded", cfg)
		}
	}
	if _, err := New(nil, time.Second); err == nil {
		t.Fatal("New accepted nil server")
	}
}

func TestAppCloseOnceWithoutRun(t *testing.T) {
	calls := 0
	a := &App{cleanup: func() { calls++ }}
	var callers sync.WaitGroup
	for range 10 {
		callers.Go(a.Close)
	}
	callers.Wait()
	a.Close()
	if calls != 1 {
		t.Fatalf("cleanup calls = %d, want 1", calls)
	}
	// Apps created directly with New have no external dependency cleanup.
	(&App{}).Close()
}

func TestAppHello(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	a, err := newHTTPTestApp(config.Config{
		HTTP: httpserver.Config{Addr: addr}, ShutdownTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Run = %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("Run did not stop")
		}
	})
	client := &http.Client{Timeout: time.Second}
	for {
		resp, err := client.Get("http://" + addr + "/hello")
		if err == nil {
			defer resp.Body.Close()
			var body struct {
				Code int `json:"code"`
				Data struct {
					Message string `json:"message"`
				} `json:"data"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusOK || body.Code != 0 || body.Data.Message != "hello" {
				t.Fatalf("GET /hello = %d, %+v", resp.StatusCode, body)
			}
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("HTTP server not ready: %v", err)
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func TestRunStartupFailureAndCanceledContext(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	a, err := newHTTPTestApp(config.Config{
		HTTP: httpserver.Config{Addr: listener.Addr().String()}, ShutdownTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	calls := 0
	a.cleanup = func() { calls++ }
	if err := a.Run(ctx); err == nil {
		t.Fatal("Run on occupied port succeeded")
	}
	a.Close()
	if calls != 1 {
		t.Fatalf("cleanup calls after startup failure = %d, want 1", calls)
	}
	canceled, stop := context.WithCancel(context.Background())
	stop()
	a, err = newHTTPTestApp(config.Config{
		HTTP: httpserver.Config{Addr: listener.Addr().String()}, ShutdownTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	calls = 0
	a.cleanup = func() { calls++ }
	if err := a.Run(canceled); err != nil {
		t.Fatalf("already canceled Run = %v", err)
	}
	a.Close()
	if calls != 1 {
		t.Fatalf("cleanup calls after early cancellation = %d, want 1", calls)
	}
}

func TestRunShutdown(t *testing.T) {
	for _, force := range []bool{false, true} {
		name := "drain"
		if force {
			name = "timeout"
		}
		t.Run(name, func(t *testing.T) {
			// Reserve an ephemeral address, then release it for the real server.
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			addr := listener.Addr().String()
			listener.Close()
			entered := make(chan struct{})
			release := make(chan struct{})
			defer close(release)
			server, err := httpserver.New(httpserver.Config{Addr: addr}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(entered)
				select {
				case <-release:
				case <-r.Context().Done():
				}
			}))
			if err != nil {
				t.Fatal(err)
			}
			a, err := New(server, 250*time.Millisecond)
			if err != nil {
				t.Fatal(err)
			}
			cleaned := make(chan struct{})
			a.cleanup = func() { close(cleaned) }
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- a.Run(ctx) }()
			client := &http.Client{Timeout: 2 * time.Second}
			requestDone := make(chan error, 1)
			go func() {
				deadline := time.Now().Add(2 * time.Second)
				for {
					resp, err := client.Get("http://" + addr)
					if err == nil {
						resp.Body.Close()
					}
					if err == nil || time.Now().After(deadline) {
						requestDone <- err
						return
					}
					select {
					case <-entered:
						requestDone <- err
						return
					default:
					}
					time.Sleep(5 * time.Millisecond)
				}
			}()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("handler did not start")
			}
			cancel()
			if !force {
				select {
				case <-cleaned:
					t.Fatal("dependencies closed before handler finished")
				case err := <-done:
					t.Fatalf("Run returned before handler finished: %v", err)
				case <-time.After(20 * time.Millisecond):
				}
				release <- struct{}{}
			}
			select {
			case err := <-done:
				if force && !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("Run = %v, want shutdown deadline", err)
				}
				if !force && err != nil {
					t.Fatalf("Run = %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("Run did not stop")
			}
			select {
			case <-cleaned:
			default:
				t.Fatal("Run returned without releasing dependencies")
			}
			a.Close()
			select {
			case err := <-requestDone:
				if !force && err != nil {
					t.Fatalf("request = %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("request did not finish")
			}
		})
	}
}

// HTTP lifecycle tests assemble transport dependencies without external services.
func newHTTPTestApp(cfg config.Config) (*App, error) {
	server, err := newHTTPServer(cfg.HTTP, &httpDependencies{
		Hello: service.NewHelloService(db.NewHelloRepo()),
	})
	if err != nil {
		return nil, err
	}
	return New(server, cfg.ShutdownTimeout)
}
