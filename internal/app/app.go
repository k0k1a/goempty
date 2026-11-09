// Package app assembles application dependencies and manages their lifecycle.
package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

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
	shutdownTimeout time.Duration
	cleanup         func()
	closeOnce       sync.Once
}

// New assembles an App without starting goroutines or opening listeners.
func New(server *httpserver.Server, jobs *scheduler.Scheduler, shutdownTimeout time.Duration) (*App, error) {
	if server == nil {
		return nil, errors.New("app: HTTP server is required")
	}
	if shutdownTimeout <= 0 {
		return nil, errors.New("app: shutdown timeout must be positive")
	}
	if jobs == nil {
		return nil, errors.New("app: scheduler is required")
	}
	return &App{httpServer: server, scheduler: jobs, shutdownTimeout: shutdownTimeout}, nil
}

// Close releases application dependencies once. Call it after Run returns,
// or without calling Run. To stop a running app, cancel the context passed to Run.
// Cleanup failures are logged by the dependency providers.
func (a *App) Close() {
	a.closeOnce.Do(func() {
		if a.cleanup != nil {
			a.cleanup()
		}
	})
}

// Run serves until ctx is canceled or the HTTP server exits. Cancellation is a
// normal exit; startup and shutdown failures are returned. Call Run only once.
// The caller owns process signals. Run waits for HTTP and task shutdown, then releases
// application dependencies before returning, including on early cancellation.
// Shutdown timeout is returned as an error; non-cooperative handlers may outlive it.
func (a *App) Run(ctx context.Context) error {
	defer a.Close()
	if ctx.Err() != nil {
		return nil
	}
	if err := a.scheduler.Start(ctx); err != nil {
		return fmt.Errorf("app: start scheduler: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- a.httpServer.Start() }()

	var runErr error
	finished := false
	select {
	case runErr = <-done:
		finished = true
	case <-ctx.Done():
	}

	// Preserve context values while giving draining requests a fresh deadline.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), a.shutdownTimeout)
	defer cancel()
	log.Info(shutdownCtx, "Shutting down application")
	jobsDone := make(chan error, 1)
	go func() { jobsDone <- a.scheduler.Stop(shutdownCtx) }()
	stopErr := a.httpServer.Stop(shutdownCtx)
	jobsErr := <-jobsDone
	if !finished {
		runErr = <-done
	}
	if err := errors.Join(runErr, stopErr, jobsErr); err != nil {
		return fmt.Errorf("app: run: %w", err)
	}
	return nil
}
