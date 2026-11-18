// Package async executes named, process-local tasks using a bounded Ants pool.
package async

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"goempty/pkg/log"

	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"
)

var (
	ErrNotStarted = errors.New("async: pool has not started")
	ErrClosed     = errors.New("async: pool is closed")
	ErrOverloaded = errors.New("async: blocked submitter limit reached")
)

// Handler must propagate ctx and cooperate with cancellation. Errors are logged
// by the pool; tasks are never retried or persisted.
type Handler func(context.Context) error

// Pool is created with New and owned by the application. It cannot be restarted.
type Pool struct {
	mu      sync.Mutex
	cfg     Config
	engine  *ants.Pool
	runCtx  context.Context
	cancel  context.CancelFunc
	stopped bool
	done    chan struct{}
	pending sync.WaitGroup
}

// New validates settings without starting pool workers or maintenance goroutines.
func New(cfg Config) (*Pool, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.Concurrency == 0 {
		cfg.Concurrency = 4
	}
	if cfg.MaxBlockingTasks == 0 {
		cfg.MaxBlockingTasks = 1000
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &Pool{cfg: cfg, done: make(chan struct{})}, nil
}

// Start creates the owned pool. Repeated calls while running are no-ops.
// The owner must call Stop, including after cancellation of ctx.
func (p *Pool) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped {
		return ErrClosed
	}
	if p.engine != nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	engine, err := ants.NewPool(p.cfg.Concurrency, ants.WithMaxBlockingTasks(p.cfg.MaxBlockingTasks))
	if err != nil {
		return fmt.Errorf("async: start: %w", err)
	}
	p.engine = engine
	p.runCtx, p.cancel = context.WithCancel(ctx)
	return nil
}

// Submit waits for a worker when all workers are busy. Once MaxBlockingTasks
// callers are waiting, further submissions fail with ErrOverloaded. The result
// describes admission, not execution. Native Ants waiting has no timeout and
// is not interrupted by cancellation of ctx; Stop wakes blocked submitters.
// Execution retains ctx values but uses the pool cancellation and task timeout.
// Captured data must not be mutated concurrently; never capture a Gin Context
// or response writer. Saturated workers must not synchronously submit to this
// same pool: nested blocking submissions can deadlock.
func (p *Pool) Submit(ctx context.Context, name string, handler Handler) error {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(name) != name || handler == nil {
		return errors.New("async: a task name without surrounding whitespace and handler are required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return ErrClosed
	}
	if p.engine == nil {
		p.mu.Unlock()
		return ErrNotStarted
	}
	engine, runCtx := p.engine, p.runCtx
	if err := runCtx.Err(); err != nil {
		p.mu.Unlock()
		return err
	}
	// Account for both the submission and its potential execution before Stop
	// can begin waiting. A rejected submission rolls back the execution count.
	p.pending.Add(2)
	p.mu.Unlock()
	defer p.pending.Done()
	// Never hold the lifecycle mutex while waiting for an Ants worker.
	err := engine.Submit(func() {
		defer p.pending.Done()
		taskCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), p.cfg.Timeout)
		stopCancel := context.AfterFunc(runCtx, cancel)
		defer stopCancel()
		defer cancel()
		if runCtx.Err() != nil {
			cancel()
		}
		p.execute(taskCtx, name, handler)
	})
	if err != nil {
		p.pending.Done()
	}
	if errors.Is(err, ants.ErrPoolOverload) {
		return ErrOverloaded
	}
	if errors.Is(err, ants.ErrPoolClosed) {
		return ErrClosed
	}
	if err != nil {
		return fmt.Errorf("async: submit %q: %w", name, err)
	}
	return nil
}

// Stop closes admission, wakes blocked submitters, cancels tasks and waits for
// submissions and handlers to exit. A timeout bounds only this wait; Stop can
// be called again. Go cannot
// forcibly terminate handlers that ignore cancellation.
func (p *Pool) Stop(ctx context.Context) error {
	p.mu.Lock()
	if !p.stopped {
		p.stopped = true
		if p.engine == nil {
			close(p.done)
		} else {
			// Close Ants synchronously before cancellation can free a busy worker
			// and allow it to accept a waiting submission during shutdown.
			p.engine.Release()
			p.cancel()
			// One owned waiter allows concurrent/repeated Stop after caller timeout.
			go func() {
				p.pending.Wait()
				close(p.done)
			}()
		}
	}
	done := p.done
	p.mu.Unlock()
	select {
	case <-done:
		return nil
	default:
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("async: stop: %w", ctx.Err())
	}
}

func (p *Pool) execute(ctx context.Context, name string, handler Handler) {
	started := time.Now()
	defer func() {
		if value := recover(); value != nil {
			log.Error(ctx, "Async task panicked", zap.String("task_name", name), zap.Any("panic", value),
				zap.Duration("duration", time.Since(started)), zap.ByteString("stack", debug.Stack()))
		}
	}()
	log.Info(ctx, "Async task started", zap.String("task_name", name))
	if err := handler(ctx); err != nil {
		log.Error(ctx, "Async task failed", zap.String("task_name", name), zap.Error(err), zap.Duration("duration", time.Since(started)))
		return
	}
	log.Info(ctx, "Async task completed", zap.String("task_name", name), zap.Duration("duration", time.Since(started)))
}
