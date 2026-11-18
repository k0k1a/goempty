package async

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"
)

func testPool(t *testing.T, cfg Config) *Pool {
	t.Helper()
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := p.Stop(ctx); err != nil {
			t.Error(err)
		}
	})
	return p
}

func await(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("task did not finish")
	}
}

func TestPoolCapacityAndRequestLifetime(t *testing.T) {
	p := testPool(t, Config{Concurrency: 1})
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), "trace_id", "trace-test"))
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	defer close(release)
	if err := p.Submit(ctx, "hold", func(ctx context.Context) error {
		close(entered)
		<-release
		if ctx.Err() != nil || ctx.Value("trace_id") != "trace-test" {
			t.Error("request lifetime or metadata leaked")
		}
		close(done)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	await(t, entered)
	cancel()
	release <- struct{}{}
	await(t, done)
}

func TestPoolShutdownTimeoutAndRepeatedStop(t *testing.T) {
	p := testPool(t, Config{Concurrency: 1})
	canceled, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	if err := p.Submit(context.Background(), "shutdown", func(ctx context.Context) error {
		<-ctx.Done()
		close(canceled)
		<-release
		return ctx.Err()
	}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := p.Stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Stop = %v", err)
	}
	await(t, canceled)
	if err := p.Submit(context.Background(), "late", func(context.Context) error { return nil }); !errors.Is(err, ErrClosed) {
		t.Fatalf("Submit = %v", err)
	}
	release <- struct{}{}
	var callers sync.WaitGroup
	for range 4 {
		callers.Go(func() {
			if err := p.Stop(context.Background()); err != nil {
				t.Error(err)
			}
		})
	}
	callers.Wait()
	if err := p.Start(context.Background()); !errors.Is(err, ErrClosed) {
		t.Fatalf("restart = %v", err)
	}
}

func TestPoolTimeoutErrorAndPanicRecovery(t *testing.T) {
	p := testPool(t, Config{Concurrency: 3, Timeout: 20 * time.Millisecond})
	done := make(chan struct{})
	if err := p.Submit(context.Background(), "timeout", func(ctx context.Context) error {
		<-ctx.Done()
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Error(ctx.Err())
		}
		close(done)
		return ctx.Err()
	}); err != nil {
		t.Fatal(err)
	}
	if err := p.Submit(context.Background(), "panic", func(context.Context) error { panic("test panic") }); err != nil {
		t.Fatal(err)
	}
	if err := p.Submit(context.Background(), "error", func(context.Context) error { return errors.New("test error") }); err != nil {
		t.Fatal(err)
	}
	await(t, done)
}

func TestPoolValidationAndBeforeStart(t *testing.T) {
	invalid := []Config{{Concurrency: -1}, {Timeout: -1}, {MaxBlockingTasks: -1}}
	if strconv.IntSize == 64 {
		tooLarge := int64(1) << 32
		invalid = append(invalid, Config{Concurrency: int(tooLarge)})
	}
	for _, cfg := range invalid {
		if _, err := New(cfg); err == nil {
			t.Fatalf("New(%+v) succeeded", cfg)
		}
	}
	p, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Submit(context.Background(), "early", func(context.Context) error { return nil }); !errors.Is(err, ErrNotStarted) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.Start(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := p.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := p.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	p = testPool(t, Config{})
	if err := p.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := p.Submit(context.Background(), "", func(context.Context) error { return nil }); err == nil {
		t.Fatal("empty name accepted")
	}
	if err := p.Submit(context.Background(), "nil", nil); err == nil {
		t.Fatal("nil handler accepted")
	}
	if err := p.Submit(ctx, "canceled", func(context.Context) error { t.Error("canceled task ran"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func waitForSubmitters(t *testing.T, p *Pool, count int) {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for p.engine.Waiting() != count {
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatalf("waiting submitters = %d, want %d", p.engine.Waiting(), count)
		}
	}
}

func TestPoolBlocksUntilWorkerAvailableAndRejectsAtWaitingLimit(t *testing.T) {
	p := testPool(t, Config{Concurrency: 1, MaxBlockingTasks: 1})
	release := make(chan struct{})
	defer close(release)
	if err := p.Submit(context.Background(), "hold", func(context.Context) error { <-release; return nil }); err != nil {
		t.Fatal(err)
	}
	submitted, ran := make(chan error, 1), make(chan struct{})
	go func() {
		submitted <- p.Submit(context.Background(), "waiting", func(context.Context) error { close(ran); return nil })
	}()
	waitForSubmitters(t, p, 1)
	if err := p.Submit(context.Background(), "overflow", func(context.Context) error { t.Error("overflow ran"); return nil }); !errors.Is(err, ErrOverloaded) {
		t.Fatalf("Submit = %v", err)
	}
	select {
	case err := <-submitted:
		t.Fatalf("waiting submission returned early: %v", err)
	default:
	}
	release <- struct{}{}
	select {
	case err := <-submitted:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("submission did not resume")
	}
	await(t, ran)
}

func TestPoolStopWakesBlockedSubmitters(t *testing.T) {
	p := testPool(t, Config{Concurrency: 1, MaxBlockingTasks: 3})
	if err := p.Submit(context.Background(), "hold", func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }); err != nil {
		t.Fatal(err)
	}
	submitted := make(chan error, 3)
	for range 3 {
		go func() {
			submitted <- p.Submit(context.Background(), "waiting", func(context.Context) error { t.Error("blocked task ran after Stop"); return nil })
		}()
	}
	waitForSubmitters(t, p, 3)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := p.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		select {
		case err := <-submitted:
			if !errors.Is(err, ErrClosed) {
				t.Fatalf("blocked Submit = %v", err)
			}
		case <-ctx.Done():
			t.Fatal("blocked submission not released")
		}
	}
}

func TestPoolDefaultBlockingLimit(t *testing.T) {
	p, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	if p.cfg.MaxBlockingTasks != 1000 {
		t.Fatalf("MaxBlockingTasks = %d", p.cfg.MaxBlockingTasks)
	}
}

func TestPoolClosesAdmissionBeforeCancelingTasks(t *testing.T) {
	p := testPool(t, Config{})
	cancel := p.cancel
	// Observe cancellation at the exact point Stop signals handlers, rather than
	// depending on which goroutine the scheduler happens to run first.
	p.cancel = func() {
		if !p.engine.IsClosed() {
			t.Error("tasks canceled while pool still accepts waiting submissions")
		}
		cancel()
	}
	if err := p.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}
