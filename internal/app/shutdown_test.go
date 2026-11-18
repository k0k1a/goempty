package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"goempty/config"
	"goempty/pkg/async"
	"goempty/pkg/event"
	"goempty/pkg/httpserver"
)

func TestShutdownRunningAppSharesErrorAndCleansUpOnce(t *testing.T) {
	a, err := newHTTPTestApp(config.Config{HTTP: httpserver.Config{Addr: "127.0.0.1:0"}, ShutdownTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	canceled, release, entered := make(chan struct{}), make(chan struct{}), make(chan struct{})
	defer close(release)
	if err := a.eventBus.Subscribe("shutdown", testEventHandler{"hold", func(ctx context.Context, _ event.Event) error {
		close(entered)
		<-ctx.Done()
		close(canceled)
		<-release
		return ctx.Err()
	}}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	a.cleanup = func() { calls++ }
	runCtx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()
	runDone := make(chan error, 1)
	go func() { runDone <- a.Run(runCtx) }()
	deadline := time.After(time.Second)
	for {
		err := a.eventBus.Publish(context.Background(), testEvent("shutdown"))
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
	select {
	case <-entered:
	case <-deadline:
		t.Fatal("event did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	shutdownErr := a.Shutdown(ctx)
	if !errors.Is(shutdownErr, context.DeadlineExceeded) {
		t.Fatalf("Shutdown = %v", shutdownErr)
	}
	select {
	case <-canceled:
	default:
		t.Fatal("Shutdown did not cancel task")
	}
	select {
	case err := <-runDone:
		if !errors.Is(err, shutdownErr) {
			t.Fatalf("Run = %v, want shared shutdown error %v", err, shutdownErr)
		}
	case <-time.After(time.Second):
		t.Fatal("Shutdown did not stop Run")
	}
	if err := a.Shutdown(context.Background()); err != shutdownErr {
		t.Fatalf("repeated Shutdown = %v", err)
	}
	if calls != 1 {
		t.Fatalf("cleanup calls = %d", calls)
	}
	release <- struct{}{}
	stopCtx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if err := a.taskPool.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
}

func TestShutdownConcurrentWithRunStart(t *testing.T) {
	a, err := newHTTPTestApp(config.Config{HTTP: httpserver.Config{Addr: "127.0.0.1:0"}, ShutdownTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	runDone := make(chan error, 1)
	go func() { runDone <- a.Run(context.Background()) }()
	if err := a.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-runDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit")
	}
	if err := a.taskPool.Start(context.Background()); !errors.Is(err, async.ErrClosed) {
		t.Fatalf("pool not closed: %v", err)
	}
}

func TestRunSchedulerStartFailureShutsDown(t *testing.T) {
	a, err := newHTTPTestApp(config.Config{HTTP: httpserver.Config{Addr: "127.0.0.1:0"}, ShutdownTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.scheduler.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	cleaned := false
	a.cleanup = func() { cleaned = true }
	if err := a.Run(context.Background()); err == nil {
		t.Fatal("Run accepted a stopped scheduler")
	}
	if !cleaned {
		t.Fatal("startup failure did not release resources")
	}
	if err := a.taskPool.Start(context.Background()); !errors.Is(err, async.ErrClosed) {
		t.Fatalf("startup left pool running: %v", err)
	}
}
