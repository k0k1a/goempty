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
	"goempty/internal/entity"
	"goempty/internal/job"
	"goempty/internal/repo/db"
	"goempty/internal/service"
	"goempty/pkg/async"
	"goempty/pkg/event"
	"goempty/pkg/httpserver"
	"goempty/pkg/scheduler"
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
	if _, err := New(nil, nil, nil, nil, nil, time.Second); err == nil {
		t.Fatal("New accepted nil server")
	}
}

func TestAppShutdownOnceWithoutRun(t *testing.T) {
	calls := 0
	a, err := newHTTPTestApp(config.Config{HTTP: httpserver.Config{Addr: "127.0.0.1:0"}, ShutdownTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	a.cleanup = func() { calls++ }
	var callers sync.WaitGroup
	for range 10 {
		callers.Go(func() {
			if err := a.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
	}
	callers.Wait()
	if err := a.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("cleanup calls = %d, want 1", calls)
	}
	if err := a.Run(context.Background()); err == nil {
		t.Fatal("Run accepted a shut down application")
	}
	if err := a.taskPool.Start(context.Background()); !errors.Is(err, async.ErrClosed) {
		t.Fatalf("pool after shutdown = %v", err)
	}
}

func TestRunWaitsForTasksBeforeCleanup(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		t.Run(map[bool]string{false: "drain", true: "timeout"}[timeout], func(t *testing.T) {
			a, err := newHTTPTestApp(config.Config{
				HTTP: httpserver.Config{Addr: "127.0.0.1:0"}, ShutdownTimeout: 150 * time.Millisecond,
			})
			if err != nil {
				t.Fatal(err)
			}
			entered, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			defer close(release)
			if err := a.scheduler.Register(testScheduledTask{func(ctx context.Context) error {
				close(entered)
				<-ctx.Done()
				close(canceled)
				<-release
				return ctx.Err()
			},
			}); err != nil {
				t.Fatal(err)
			}
			cleaned := make(chan struct{})
			a.cleanup = func() { close(cleaned) }
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- a.Run(ctx) }()
			t.Cleanup(func() {
				stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
				defer stopCancel()
				if err := a.scheduler.Stop(stopCtx); err != nil {
					t.Errorf("task cleanup: %v", err)
				}
			})
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("scheduled task did not start")
			}
			cancel()
			select {
			case <-canceled:
			case <-time.After(time.Second):
				t.Fatal("task context was not canceled")
			}
			if !timeout {
				select {
				case <-cleaned:
					t.Fatal("dependencies released while task was running")
				case <-done:
					t.Fatal("Run returned while task was running")
				default:
				}
				release <- struct{}{}
			}
			select {
			case err := <-done:
				if timeout && !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("Run = %v, want task shutdown deadline", err)
				}
				if !timeout && err != nil {
					t.Fatalf("Run = %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("Run did not stop")
			}
			select {
			case <-cleaned:
			default:
				t.Fatal("dependencies were not released")
			}
		})
	}
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
	if err := a.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
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
	if err := a.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
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
			jobs, err := scheduler.New(scheduler.Config{})
			if err != nil {
				t.Fatal(err)
			}
			tasks, events := newTestTasks(t, async.Config{})
			a, err := New(server, jobs, tasks, events, nil, 250*time.Millisecond)
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
			if err := a.Shutdown(context.Background()); !force && err != nil {
				t.Fatal(err)
			}
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
	tasks, err := async.New(cfg.Async)
	if err != nil {
		return nil, err
	}
	events, err := event.New(tasks)
	if err != nil {
		return nil, err
	}
	helloService := service.NewHelloService(db.NewHelloRepo(), events)
	server, err := newHTTPServer(cfg.HTTP, &httpDependencies{HelloService: helloService})
	if err != nil {
		return nil, err
	}
	jobs, err := newScheduler(&jobDependencies{HelloService: helloService})
	if err != nil {
		return nil, err
	}
	return newApp(server, jobs, tasks, events, nil, &eventDependencies{HelloService: helloService}, cfg.ShutdownTimeout, nil, nil, nil)
}

func newTestTasks(t *testing.T, cfg async.Config) (*async.Pool, *event.Bus) {
	t.Helper()
	tasks, err := async.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	events, err := event.New(tasks)
	if err != nil {
		t.Fatal(err)
	}
	return tasks, events
}

func TestRunWaitsForEventsBeforeCleanup(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		t.Run(map[bool]string{false: "drain", true: "timeout"}[timeout], func(t *testing.T) {
			a, err := newHTTPTestApp(config.Config{HTTP: httpserver.Config{Addr: "127.0.0.1:0"}, ShutdownTimeout: 100 * time.Millisecond})
			if err != nil {
				t.Fatal(err)
			}
			canceled, release, last := make(chan struct{}), make(chan struct{}), make(chan struct{})
			defer close(release)
			if err := a.eventBus.Subscribe("lifecycle", testEventHandler{"hold", func(ctx context.Context, _ event.Event) error {
				<-ctx.Done()
				close(canceled)
				<-release
				return ctx.Err()
			}}); err != nil {
				t.Fatal(err)
			}
			if err := a.eventBus.Subscribe("lifecycle", testEventHandler{"last", func(context.Context, event.Event) error { close(last); return nil }}); err != nil {
				t.Fatal(err)
			}
			cleaned := make(chan struct{})
			a.cleanup = func() { close(cleaned) }
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- a.Run(ctx) }()
			deadline := time.After(time.Second)
			for {
				err = a.eventBus.Publish(context.Background(), testEvent("lifecycle"))
				if err == nil {
					break
				}
				if !errors.Is(err, event.ErrNotStarted) {
					t.Fatal(err)
				}
				select {
				case <-deadline:
					t.Fatal("app did not start")
				case <-time.After(time.Millisecond):
				}
			}
			cancel()
			select {
			case <-canceled:
			case <-time.After(time.Second):
				t.Fatal("event not canceled")
			}
			if !timeout {
				select {
				case <-cleaned:
					t.Fatal("cleanup before event completed")
				default:
				}
				release <- struct{}{}
			}
			select {
			case err := <-done:
				if timeout && !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("Run = %v", err)
				}
				if !timeout && err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("app did not stop")
			}
			select {
			case <-cleaned:
			default:
				t.Fatal("cleanup missing")
			}
			if timeout {
				release <- struct{}{}
			}
			stopCtx, stop := context.WithTimeout(context.Background(), time.Second)
			defer stop()
			if err := a.taskPool.Stop(stopCtx); err != nil {
				t.Fatal(err)
			}
			select {
			case <-last:
			default:
				t.Fatal("second event handler did not run")
			}
		})
	}
}

type testScheduledTask struct {
	run func(context.Context) error
}

func (testScheduledTask) Name() string           { return "lifecycle" }
func (testScheduledTask) Spec() string           { return "@every 1s" }
func (testScheduledTask) Timeout() time.Duration { return 0 }

func (task testScheduledTask) Run(ctx context.Context) error { return task.run(ctx) }

type testEvent string

func (e testEvent) Name() string { return string(e) }

type testEventHandler struct {
	name   string
	handle func(context.Context, event.Event) error
}

func (h testEventHandler) Name() string { return h.name }

func (h testEventHandler) Handle(ctx context.Context, e event.Event) error {
	return h.handle(ctx, e)
}

type unexpectedHelloPublisher struct{}

func (unexpectedHelloPublisher) Publish(context.Context, event.Event) error {
	panic("HTTP Hello must not publish events")
}

func newTestHelloService(repo db.HelloRepository) *service.HelloService {
	return service.NewHelloService(repo, unexpectedHelloPublisher{})
}

func TestHelloJobEventFlow(t *testing.T) {
	a, err := newHTTPTestApp(config.Config{
		HTTP: httpserver.Config{Addr: "127.0.0.1:0"}, ShutdownTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := a.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})

	helloService := service.NewHelloService(db.NewHelloRepo(), a.eventBus)
	if err := a.scheduler.Register(job.NewHelloJob(helloService)); err == nil {
		t.Fatal("Hello Job was not registered during assembly")
	}
	if err := registerEvents(a.eventBus, &eventDependencies{HelloService: helloService}); !errors.Is(err, event.ErrDuplicateHandler) {
		t.Fatalf("Hello event controller registration = %v", err)
	}

	received := make(chan event.Event, 1)
	if err := a.eventBus.Subscribe(entity.HelloGreetingRequested, testEventHandler{
		name: "probe",
		handle: func(ctx context.Context, e event.Event) error {
			select {
			case received <- e:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.taskPool.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	a.eventBus.Start()
	// Invoke the demo job directly so the test does not depend on a minute boundary.
	if err := job.NewHelloJob(helloService).Run(t.Context()); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-received:
		if greeting, ok := e.(entity.HelloGreetingEvent); !ok || greeting.Message != "hello" {
			t.Fatalf("greeting event = %#v", e)
		}
	case <-time.After(time.Second):
		t.Fatal("greeting event was not dispatched")
	}
}
