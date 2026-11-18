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

// Handler processes an event, propagating ctx to dependencies. It must respond
// to cancellation. Errors and panics are logged and do not prevent later handlers.
type Handler func(context.Context, Event) error

type registration struct {
	name    string
	handler Handler
}

// Bus supports one or more handlers per event name. It does not own the pool.
// Register handlers before Start. Different events may run concurrently;
// publication order across events is not guaranteed.
type Bus struct {
	mu       sync.RWMutex
	pool     *async.Pool
	handlers map[string][]registration
	started  bool
}

func New(pool *async.Pool) (*Bus, error) {
	if pool == nil {
		return nil, errors.New("event: task pool is required")
	}
	return &Bus{pool: pool, handlers: make(map[string][]registration)}, nil
}

// Register adds a handler in execution order. Names are stable business labels,
// not payloads or resource IDs. Handler names must be unique within an event.
func (b *Bus) Register(eventName, handlerName string, handler Handler) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.started {
		return ErrRegistrationClosed
	}
	if !validName(eventName) || !validName(handlerName) || handler == nil {
		return errors.New("event: event name, handler name without surrounding whitespace and handler are required")
	}
	for _, existing := range b.handlers[eventName] {
		if existing.name == handlerName {
			return fmt.Errorf("%w: %s/%s", ErrDuplicateHandler, eventName, handlerName)
		}
	}
	b.handlers[eventName] = append(b.handlers[eventName], registration{name: handlerName, handler: handler})
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
func (b *Bus) Publish(ctx context.Context, event Event) error {
	b.mu.RLock()
	if !b.started {
		b.mu.RUnlock()
		return ErrNotStarted
	}
	handlers := b.handlers[event.Name]
	b.mu.RUnlock()
	if len(handlers) == 0 {
		return fmt.Errorf("%w: %s", ErrNoHandlers, event.Name)
	}
	return b.pool.Submit(ctx, event.Name, func(ctx context.Context) error {
		for _, handler := range handlers {
			execute(ctx, event, handler)
		}
		return nil
	})
}

func validName(name string) bool {
	return name != "" && strings.TrimSpace(name) == name
}

func execute(ctx context.Context, event Event, handler registration) {
	started := time.Now()
	fields := []zap.Field{zap.String("event_name", event.Name), zap.String("handler_name", handler.name)}
	defer func() {
		if value := recover(); value != nil {
			log.Error(ctx, "Event handler panicked", append(fields, zap.Any("panic", value),
				zap.Duration("duration", time.Since(started)), zap.ByteString("stack", debug.Stack()))...)
		}
	}()
	log.Info(ctx, "Event handler started", fields...)
	if err := handler.handler(ctx, event); err != nil {
		log.Error(ctx, "Event handler failed", append(fields, zap.Error(err), zap.Duration("duration", time.Since(started)))...)
		return
	}
	log.Info(ctx, "Event handler completed", append(fields, zap.Duration("duration", time.Since(started)))...)
}
