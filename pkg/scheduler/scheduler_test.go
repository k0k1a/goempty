package scheduler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConfigurationAndRegistration(t *testing.T) {
	if _, err := New(Config{Timezone: "Missing/Zone"}); err == nil {
		t.Fatal("invalid timezone accepted")
	}
	s := newTestScheduler(t)
	if s.cron.Location() != time.Local {
		t.Fatal("empty timezone must follow time.Local")
	}
	valid := Task{Name: "valid", Spec: "0 * * * *", Handler: func(context.Context) error { return nil }}
	for _, task := range []Task{
		{Name: "", Spec: valid.Spec, Handler: valid.Handler},
		{Name: " padded ", Spec: valid.Spec, Handler: valid.Handler},
		{Name: "nil", Spec: valid.Spec},
		{Name: "negative", Spec: valid.Spec, Handler: valid.Handler, Timeout: -time.Second},
		{Name: "bad-spec", Spec: "invalid", Handler: valid.Handler},
		{Name: "six-fields", Spec: "* * * * * *", Handler: valid.Handler},
	} {
		if err := s.Register(task); err == nil {
			t.Fatalf("invalid task %q accepted", task.Name)
		}
	}
	if err := s.Register(valid); err != nil {
		t.Fatal(err)
	}
	if err := s.Register(valid); err == nil {
		t.Fatal("duplicate name accepted")
	}
	if len(s.cron.Entries()) != 1 {
		t.Fatal("failed registrations changed scheduled entries")
	}
	startTestScheduler(t, s, context.Background())
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("repeated Start: %v", err)
	}
	if err := s.Register(Task{Name: "late", Spec: valid.Spec, Handler: valid.Handler}); err == nil {
		t.Fatal("registration after Start accepted")
	}
}

func TestTimezoneDefaultsAndOverrides(t *testing.T) {
	previous := time.Local
	t.Cleanup(func() { time.Local = previous })
	time.Local = time.FixedZone("TestLocal", 9*60*60)
	for _, tt := range []struct {
		name string
		cfg  Config
		zone string
		hour int
	}{
		{name: "default follows process", zone: "TestLocal", hour: 9},
		{name: "explicit UTC", cfg: Config{Timezone: "UTC"}, zone: "UTC", hour: 0},
		{name: "explicit Shanghai", cfg: Config{Timezone: "Asia/Shanghai"}, zone: "Asia/Shanghai", hour: 8},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, err := New(tt.cfg)
			if err != nil {
				t.Fatal(err)
			}
			loc := s.cron.Location()
			if loc.String() != tt.zone {
				t.Fatalf("timezone = %s, want %s", loc, tt.zone)
			}
			instant := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
			if hour := instant.In(loc).Hour(); hour != tt.hour {
				t.Fatalf("local hour = %d, want %d", hour, tt.hour)
			}
		})
	}
}

func TestLifecycle(t *testing.T) {
	s := newTestScheduler(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Start(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Start canceled context = %v", err)
	}
	if err := s.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("repeated Stop: %v", err)
	}
	if err := s.Start(context.Background()); err == nil {
		t.Fatal("restart after Stop accepted")
	}
	if err := s.Register(Task{}); err == nil {
		t.Fatal("registration after Stop accepted")
	}
}

func TestConcurrentDuplicateRegistration(t *testing.T) {
	s := newTestScheduler(t)
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			if err := s.Register(Task{Name: "same", Spec: "@hourly", Handler: func(context.Context) error { return nil }}); err == nil {
				accepted.Add(1)
			}
		})
	}
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatalf("successful registrations = %d", accepted.Load())
	}
}

func TestOverlapAndIndependentTasks(t *testing.T) {
	s := newTestScheduler(t)
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	defer close(release)
	var calls atomic.Int32
	registerTestTask(t, s, Task{Name: "blocking", Handler: func(context.Context) error {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		return nil
	}})
	var otherCalls atomic.Int32
	registerTestTask(t, s, Task{Name: "other", Handler: func(context.Context) error {
		otherCalls.Add(1)
		return nil
	}})
	startTestScheduler(t, s, context.Background())
	jobs := s.cron.Entries()
	go func() { jobs[0].Job.Run(); close(finished) }()
	awaitSignal(t, entered)
	jobs[0].Job.Run()
	jobs[1].Job.Run()
	if calls.Load() != 1 || otherCalls.Load() != 1 {
		t.Fatal("overlap was not skipped or independent task was blocked")
	}
	release <- struct{}{}
	awaitSignal(t, finished)
	jobs[0].Job.Run()
	if calls.Load() != 2 {
		t.Fatal("completed task could not run again")
	}
}

func TestPanicAndErrorDoNotDisableTask(t *testing.T) {
	s := newTestScheduler(t)
	calls := 0
	registerTestTask(t, s, Task{Name: "recover", Handler: func(context.Context) error {
		calls++
		if calls == 1 {
			panic("test panic")
		}
		return errors.New("test error")
	}})
	startTestScheduler(t, s, context.Background())
	job := s.cron.Entries()[0].Job
	for range 3 {
		job.Run()
	}
	if calls != 3 {
		t.Fatalf("runs after panic and error = %d, want 3", calls)
	}
}

func TestTaskTimeoutAndContextValues(t *testing.T) {
	s := newTestScheduler(t)
	type key struct{}
	var taskErr error
	registerTestTask(t, s, Task{Name: "timeout", Timeout: 10 * time.Millisecond, Handler: func(ctx context.Context) error {
		if ctx.Value(key{}) != "preserved" {
			t.Error("parent context value lost")
		}
		<-ctx.Done()
		taskErr = ctx.Err()
		return taskErr
	}})
	startTestScheduler(t, s, context.WithValue(context.Background(), key{}, "preserved"))
	s.cron.Entries()[0].Job.Run()
	if !errors.Is(taskErr, context.DeadlineExceeded) {
		t.Fatalf("task error = %v", taskErr)
	}
}

// Use the real cron runner here so Stop also proves it waits for tracked jobs.
func TestStopCancelsAndWaitsForRunningTask(t *testing.T) {
	s := newTestScheduler(t)
	entered, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	defer close(release)
	if err := s.Register(Task{Name: "shutdown", Spec: "@every 1s", Handler: func(ctx context.Context) error {
		close(entered)
		<-ctx.Done()
		close(canceled)
		<-release
		return ctx.Err()
	}}); err != nil {
		t.Fatal(err)
	}
	startTestScheduler(t, s, context.Background())
	awaitSignal(t, entered)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := s.Stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Stop before task finishes = %v", err)
	}
	awaitSignal(t, canceled)
	release <- struct{}{}
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	if err := s.Stop(ctx2); err != nil {
		t.Fatalf("Stop after task finishes: %v", err)
	}
}

func TestRealSchedulingAndParentCancellation(t *testing.T) {
	s := newTestScheduler(t)
	finished := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Register(Task{Name: "scheduled", Spec: "@every 1s", Handler: func(ctx context.Context) error {
		cancel()
		<-ctx.Done()
		close(finished)
		return nil
	}}); err != nil {
		t.Fatal(err)
	}
	startTestScheduler(t, s, ctx)
	awaitSignal(t, finished)
	// A queued invocation after cancellation must not enter the handler again.
	s.cron.Entries()[0].Job.Run()
}

func newTestScheduler(t *testing.T) *Scheduler {
	t.Helper()
	s, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func registerTestTask(t *testing.T, s *Scheduler, task Task) {
	t.Helper()
	task.Spec = "@every 1h"
	if err := s.Register(task); err != nil {
		t.Fatal(err)
	}
}

func startTestScheduler(t *testing.T, s *Scheduler, ctx context.Context) {
	t.Helper()
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := s.Stop(ctx); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
}

func awaitSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for task")
	}
}
