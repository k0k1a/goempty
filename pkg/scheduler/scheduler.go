// Package scheduler runs named, in-process cron tasks with cooperative cancellation.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // Keep configured zones available in minimal deployment images.

	"goempty/pkg/log"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// Handler must propagate ctx to its dependencies and return when it is canceled.
// Errors are logged once by the scheduler; tasks are not automatically retried.
type Handler func(context.Context) error

// Task defines one schedule. Timeout zero adds no execution deadline.
type Task struct {
	Name    string
	Spec    string
	Timeout time.Duration
	Handler Handler
}

// Scheduler is constructed with New. Register tasks before Start. A stopped
// scheduler cannot be restarted. Different tasks may execute concurrently.
type Scheduler struct {
	mu      sync.Mutex
	cron    *cron.Cron
	names   map[string]struct{}
	started bool
	stopped bool
	runCtx  context.Context
	cancel  context.CancelFunc
	done    context.Context
}

// New validates configuration without starting goroutines.
func New(cfg Config) (*Scheduler, error) {
	loc, err := cfg.location()
	if err != nil {
		return nil, err
	}
	return &Scheduler{
		cron:  cron.New(cron.WithLocation(loc), cron.WithLogger(cronLogger{})),
		names: make(map[string]struct{}),
	}, nil
}

// Register validates a task and rejects duplicate names or registration after Start.
// Specs use five fields (minute first), or descriptors such as @every 10m.
func (s *Scheduler) Register(task Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started || s.stopped {
		return errors.New("scheduler: registration is closed")
	}
	if task.Name == "" || strings.TrimSpace(task.Name) != task.Name {
		return errors.New("scheduler: task name must be nonempty without surrounding whitespace")
	}
	if task.Handler == nil || task.Timeout < 0 {
		return fmt.Errorf("scheduler: task %q requires a handler and nonnegative timeout", task.Name)
	}
	if _, exists := s.names[task.Name]; exists {
		return fmt.Errorf("scheduler: duplicate task %q", task.Name)
	}
	job := cron.NewChain(cron.SkipIfStillRunning(cronLogger{name: task.Name})).Then(
		cron.FuncJob(func() { s.execute(task) }),
	)
	if _, err := s.cron.AddJob(task.Spec, job); err != nil {
		return fmt.Errorf("scheduler: task %q: %w", task.Name, err)
	}
	s.names[task.Name] = struct{}{}
	return nil
}

// Start launches scheduling without blocking. Repeated calls while started are
// no-ops. ctx supplies task values, cancellation and any parent deadline.
// The owner must call Stop even if ctx has already been canceled.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return errors.New("scheduler: cannot start after stop")
	}
	if s.started {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("scheduler: start: %w", err)
	}
	s.runCtx, s.cancel = context.WithCancel(ctx)
	s.started = true
	s.cron.Start()
	return nil
}

// Stop prevents new schedules, cancels running tasks, and waits for completion.
// ctx bounds the wait, not task lifetime: Go cannot forcibly terminate handlers.
// After a timeout callers may call Stop again to continue waiting.
func (s *Scheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.stopped {
		s.stopped = true
		s.done = s.cron.Stop()
		if s.cancel != nil {
			s.cancel()
		}
	}
	done := s.done.Done()
	s.mu.Unlock()
	select {
	case <-done:
		return nil
	default:
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("scheduler: stop: %w", ctx.Err())
	}
}

func (s *Scheduler) execute(task Task) {
	ctx := s.runCtx
	if ctx.Err() != nil {
		return
	}
	if task.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, task.Timeout)
		defer cancel()
	}
	started := time.Now()
	// Recover inside SkipIfStillRunning so its execution token is always returned.
	defer func() {
		if value := recover(); value != nil {
			log.Error(ctx, "Scheduled task panicked", zap.String("task", task.Name),
				zap.Duration("duration", time.Since(started)), zap.Any("panic", value), zap.ByteString("stack", debug.Stack()))
		}
	}()
	if err := task.Handler(ctx); err != nil {
		log.Error(ctx, "Scheduled task failed", zap.String("task", task.Name),
			zap.Duration("duration", time.Since(started)), zap.Error(err))
		return
	}
	log.Info(ctx, "Scheduled task completed", zap.String("task", task.Name), zap.Duration("duration", time.Since(started)))
}

// cronLogger adapts engine and skip diagnostics to the existing project logger.
type cronLogger struct{ name string }

func (l cronLogger) Info(msg string, keysAndValues ...interface{}) {
	fields := []zap.Field{zap.String("event", msg), zap.Any("details", keysAndValues)}
	if l.name != "" {
		log.Info(context.Background(), "Scheduled task skipped", append(fields, zap.String("task", l.name))...)
		return
	}
	log.Debug(context.Background(), "Scheduler event", fields...)
}

func (l cronLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	log.Error(context.Background(), "Scheduler error", zap.String("event", msg),
		zap.Any("details", keysAndValues), zap.Error(err))
}
