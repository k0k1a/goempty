package integration_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"goempty/pkg/httpserver"
)

func TestHTTPServerLifecycle(t *testing.T) {
	s, url, done := startHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ready")
	}))
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil || string(body) != "ready" {
		t.Fatalf("response = %q, %v", body, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	waitHTTPStart(t, done)
}

func TestHTTPServerPortInUse(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	s, err := httpserver.New(httpserver.Config{Addr: listener.Addr().String()}, http.NotFoundHandler())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Start(); err == nil {
		t.Fatal("Start on occupied port succeeded")
	}
}

func TestHTTPServerShutdown(t *testing.T) {
	for _, force := range []bool{false, true} {
		name := "graceful"
		if force {
			name = "timeout"
		}
		t.Run(name, func(t *testing.T) {
			entered := make(chan struct{})
			release := make(chan struct{})
			defer close(release)
			s, url, done := startHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(entered)
				select {
				case <-release:
				case <-r.Context().Done():
				}
			}))
			requestDone := make(chan error, 1)
			go func() {
				client := &http.Client{Timeout: 3 * time.Second}
				resp, err := client.Get(url)
				if resp != nil {
					resp.Body.Close()
				}
				requestDone <- err
			}()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("handler did not start")
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if force {
				timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 30*time.Millisecond)
				defer timeoutCancel()
				if err := s.Stop(timeoutCtx); !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("Stop = %v, want context.DeadlineExceeded", err)
				}
			} else {
				stopped := make(chan error, 1)
				go func() { stopped <- s.Stop(ctx) }()
				// Start returns when the listener closes, before the active request drains.
				waitHTTPStart(t, done)
				select {
				case err := <-stopped:
					t.Fatalf("Stop returned before active request finished: %v", err)
				default:
				}
				release <- struct{}{}
				if err := <-stopped; err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-requestDone:
				if !force && err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("request did not finish after Stop")
			}
			if force {
				waitHTTPStart(t, done)
			}
		})
	}
}

func startHTTPServer(t *testing.T, handler http.Handler) (*httpserver.Server, string, <-chan error) {
	t.Helper()
	// Reserve an OS-selected port, then hand it to the server under test.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := httpserver.New(httpserver.Config{Addr: addr, ReadHeaderTimeout: time.Second}, handler)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = s.Stop(ctx)
	})
	done := make(chan error, 1)
	go func() { done <- s.Start() }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		select {
		case err := <-done:
			t.Fatalf("Start exited before readiness: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not listen: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return s, "http://" + addr, done
}

func waitHTTPStart(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not return after Stop")
	}
}
