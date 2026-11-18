// Package event dispatches named, process-local events through a shared task pool.
package event

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"goempty/pkg/async"
	"goempty/pkg/log"

	"go.uber.org/zap"
)

var (
	ErrNotStarted         = errors.New("event: bus has not started")
	ErrRegistrationClosed = errors.New("event: registration is closed")
	ErrDuplicateHandler   = errors.New("event: duplicate handler name")
	ErrNoHandlers         = errors.New("event: no handlers registered")
)

// Handler processes an event and must propagate ctx and respond to cancellation.
// Name identifies the handler for registration and logging.
type Handler interface {
	Name() string
	Handle(ctx context.Context, event Event) error
}

type subscription struct {
	name    string
	handler Handler
}

// Bus supports one or more handlers per event name. It does not own the pool.
// Subscribe handlers before Start. Different events may run concurrently;
// publication order across events is not guaranteed.
type Bus struct {
	mu            sync.RWMutex
	pool          *async.Pool
	subscriptions map[string][]subscription
	started       bool
}

func New(pool *async.Pool) (*Bus, error) {
	if pool == nil {
		return nil, errors.New("event: task pool is required")
	}
	return &Bus{pool: pool, subscriptions: make(map[string][]subscription)}, nil
}

// Subscribe adds a handler in execution order. Handler names must be unique within an event.
func (b *Bus) Subscribe(eventName string, handler Handler) error {
	if handler == nil {
		return errors.New("event: handler is required")
	}
	handlerName := handler.Name()

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.started {
		return ErrRegistrationClosed
	}
	if !validName(eventName) || !validName(handlerName) {
		return errors.New("event: event and handler names must be nonempty without surrounding whitespace")
	}
	for _, existing := range b.subscriptions[eventName] {
		if existing.name == handlerName {
			return fmt.Errorf("%w: %s/%s", ErrDuplicateHandler, eventName, handlerName)
		}
	}
	b.subscriptions[eventName] = append(b.subscriptions[eventName], subscription{name: handlerName, handler: handler})
	return nil
}

// Start freezes registration and enables publication. Start the shared pool
// before calling this method. Repeated calls are harmless.
func (b *Bus) Start() {
	b.mu.Lock()
	b.started = true
	b.mu.Unlock()
}

// Publish admits a single task for the whole event. Success means accepted,
// not completed. Handlers execute serially in registration order and share the
// event's timeout. Pool admission failures are returned to the publisher.
func (b *Bus) Publish(ctx context.Context, e Event) error {
	if e == nil {
		return errors.New("event: event is required")
	}
	eventName := e.Name()

	b.mu.RLock()
	if !b.started {
		b.mu.RUnlock()
		return ErrNotStarted
	}
	subs := b.subscriptions[eventName]
	b.mu.RUnlock()
	if len(subs) == 0 {
		return fmt.Errorf("%w: %s", ErrNoHandlers, eventName)
	}
	return b.pool.Submit(ctx, eventName, func(ctx context.Context) error {
		for _, sub := range subs {
			execute(ctx, eventName, e, sub)
		}
		return nil
	})
}

func validName(name string) bool {
	return name != "" && strings.TrimSpace(name) == name
}

func execute(ctx context.Context, eventName string, e Event, sub subscription) {
	started := time.Now()
	fields := []zap.Field{zap.String("event_name", eventName), zap.String("handler_name", sub.name)}
	defer func() {
		if value := recover(); value != nil {
			log.Error(ctx, "Event handler panicked", append(fields, zap.Any("panic", value),
				zap.Duration("duration", time.Since(started)), zap.ByteString("stack", debug.Stack()))...)
		}
	}()
	log.Info(ctx, "Event handler started", fields...)
	if err := sub.handler.Handle(ctx, e); err != nil {
		log.Error(ctx, "Event handler failed", append(fields, zap.Error(err), zap.Duration("duration", time.Since(started)))...)
		return
	}
	log.Info(ctx, "Event handler completed", append(fields, zap.Duration("duration", time.Since(started)))...)
}
