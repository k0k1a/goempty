// Package app assembles application dependencies and manages their lifecycle.
package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"goempty/pkg/async"
	"goempty/pkg/event"
	"goempty/pkg/httpserver"
	"goempty/pkg/log"
	"goempty/pkg/scheduler"

	goredis "github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"gorm.io/gorm"
)

// App owns the running components. Business dependencies are held by their
// consumers rather than exposed through App as a service registry.
type App struct {
	database        *gorm.DB
	redisClient     *goredis.Client
	mongoClient     *mongo.Client
	httpServer      *httpserver.Server
	scheduler       *scheduler.Scheduler
	taskPool        *async.Pool
	eventBus        *event.Bus
	shutdownTimeout time.Duration
	cleanup         func()
	lifecycleMu     sync.Mutex
	stopped         bool
	shutdownOnce    sync.Once
	shutdownErr     error
}

// New assembles an App without starting goroutines or opening listeners.
func New(server *httpserver.Server, jobs *scheduler.Scheduler, taskPool *async.Pool, eventBus *event.Bus, shutdownTimeout time.Duration) (*App, error) {
	if server == nil {
		return nil, errors.New("app: HTTP server is required")
	}
	if shutdownTimeout <= 0 {
		return nil, errors.New("app: shutdown timeout must be positive")
	}
	if jobs == nil {
		return nil, errors.New("app: scheduler is required")
	}
	if taskPool == nil || eventBus == nil {
		return nil, errors.New("app: task pool and event bus are required")
	}
	return &App{httpServer: server, scheduler: jobs, taskPool: taskPool, eventBus: eventBus, shutdownTimeout: shutdownTimeout}, nil
}

// Run serves until ctx is canceled or the HTTP server exits. Cancellation is a
// normal exit; startup and shutdown failures are returned. Call Run only once.
// The caller owns process signals. Run waits for HTTP and task shutdown, then releases
// application dependencies before returning, including on early cancellation.
// Shutdown timeout is returned as an error; non-cooperative handlers may outlive it.
func (a *App) Run(ctx context.Context) error {
	done, err := a.start(ctx)
	if err != nil || done == nil {
		return errors.Join(err, a.Shutdown(context.WithoutCancel(ctx)))
	}

	var runErr error
	finished := false
	select {
	case runErr = <-done:
		finished = true
	case <-ctx.Done():
	}

	shutdownErr := a.Shutdown(context.WithoutCancel(ctx))
	if !finished {
		runErr = <-done
	}
	if err := errors.Join(runErr, shutdownErr); err != nil {
		return fmt.Errorf("app: run: %w", err)
	}
	return nil
}

// start serializes startup decisions with Shutdown and rejects a stopped App.
// Run is called only once; HTTP serving runs in its own goroutine.
func (a *App) start(ctx context.Context) (<-chan error, error) {
	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()
	if a.stopped {
		return nil, errors.New("app: application is shut down")
	}
	if ctx.Err() != nil {
		return nil, nil
	}
	if err := a.taskPool.Start(ctx); err != nil {
		return nil, fmt.Errorf("app: start task pool: %w", err)
	}
	a.eventBus.Start()
	if err := a.scheduler.Start(ctx); err != nil {
		return nil, fmt.Errorf("app: start scheduler: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- a.httpServer.Start() }()
	return done, nil
}

// Shutdown stops HTTP, scheduling and async work, then releases dependencies,
// including when Run was never called or startup only partially succeeded.
// The first call controls the shared shutdown, bounded by ctx and the configured
// timeout. Concurrent callers wait for that call; all calls return its result.
// Cleanup runs once even after a timeout; non-cooperative handlers may outlive it.
// Dependency cleanup errors are logged by providers, whose cleanup API is func().
func (a *App) Shutdown(ctx context.Context) error {
	a.shutdownOnce.Do(func() {
		a.lifecycleMu.Lock()
		a.stopped = true
		a.lifecycleMu.Unlock()
		shutdownCtx, cancel := context.WithTimeout(ctx, a.shutdownTimeout)
		defer cancel()
		log.Info(shutdownCtx, "Shutting down application")
		jobsDone := make(chan error, 1)
		go func() { jobsDone <- a.scheduler.Stop(shutdownCtx) }()
		tasksDone := make(chan error, 1)
		go func() { tasksDone <- a.taskPool.Stop(shutdownCtx) }()
		stopErr := a.httpServer.Stop(shutdownCtx)
		jobsErr := <-jobsDone
		tasksErr := <-tasksDone
		a.shutdownErr = errors.Join(stopErr, jobsErr, tasksErr)
		if a.cleanup != nil {
			a.cleanup()
		}
	})
	return a.shutdownErr
}
