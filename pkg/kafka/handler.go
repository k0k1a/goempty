package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrDuplicateRoute reports a Subscribe call for a topic and event type the
// router already has a handler for.
var ErrDuplicateRoute = errors.New("kafka: duplicate route registration")

// Handler processes one decoded event for a consumer. Return nil to commit
// the record and continue. Return an error to retry the same record after a
// backoff; after MaxAttempts the record is logged and skipped. Handlers must
// dispatch promptly and respond to cancellation. Build one with NewRouter and
// Subscribe, or provide a custom implementation for full control.
type Handler interface {
	Handle(ctx context.Context, event MessageRawData) error
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(ctx context.Context, event MessageRawData) error

// Handle implements Handler.
func (f HandlerFunc) Handle(ctx context.Context, event MessageRawData) error {
	return f(ctx, event)
}

// TypedHandler processes events of one event type. Data is the type the event
// data is decoded into; Subscribe performs the decoding. Name identifies the
// handler in registration errors and is intended for logging. Implementations
// usually hold their service dependencies.
type TypedHandler[Data any] interface {
	Name() string
	Handle(ctx context.Context, event MessageRawData, data Data) error
}

// Router dispatches events by logical topic and event type. Build one with
// NewRouter, register every handler with Subscribe, and pass it to
// NewConsumer. Events with an unregistered topic and event type combination,
// and data that does not match the handler's declared type, are logged and
// skipped rather than retried.
type Router struct {
	handlers map[routeKey]routedHandler
}

type routeKey struct {
	topic     string
	eventType string
}

type routedHandler struct {
	name   string
	handle func(ctx context.Context, event MessageRawData) error
}

// NewRouter creates an empty Router.
func NewRouter() *Router {
	return &Router{handlers: make(map[routeKey]routedHandler)}
}

// Subscribe registers handler for eventType only for events from the logical
// topic. Routes must be unique per topic and event type; the same event type
// may register different handlers on different topics. Register handlers
// before passing the router to NewConsumer.
func Subscribe[Data any](router *Router, topic, eventType string, handler TypedHandler[Data]) error {
	if router == nil {
		return errors.New("kafka: router is required")
	}
	if !validRouterName(topic) || !validRouterName(eventType) {
		return errors.New("kafka: topic and event type names must be nonempty without surrounding whitespace")
	}
	routed, err := newRoutedHandler(handler)
	if err != nil {
		return err
	}
	key := routeKey{topic: topic, eventType: eventType}
	if _, exists := router.handlers[key]; exists {
		return fmt.Errorf("%w: %s/%s", ErrDuplicateRoute, topic, eventType)
	}
	router.handlers[key] = routed
	return nil
}

func newRoutedHandler[Data any](handler TypedHandler[Data]) (routedHandler, error) {
	if handler == nil {
		return routedHandler{}, errors.New("kafka: handler is required")
	}
	name := handler.Name()
	if !validRouterName(name) {
		return routedHandler{}, errors.New("kafka: handler names must be nonempty without surrounding whitespace")
	}
	return routedHandler{name: name, handle: func(ctx context.Context, event MessageRawData) error {
		var data Data
		if err := json.Unmarshal(event.rawData, &data); err != nil {
			// A data payload that does not match the handler's type cannot
			// be fixed by retrying it.
			logRejected(ctx, event, fmt.Errorf("kafka: decode data: %w", err), 0)
			return nil
		}
		return handler.Handle(ctx, event, data)
	}}, nil
}

// Handle dispatches by the event's logical topic and event type. Events
// without a registered route and data decode failures are logged and skipped;
// handler errors are returned so the consumer applies its retry policy.
func (r *Router) Handle(ctx context.Context, event MessageRawData) error {
	routed, ok := r.handlers[routeKey{topic: event.Topic, eventType: event.EventType}]
	if !ok {
		logRejected(ctx, event,
			fmt.Errorf("kafka: no handler registered for event type %q on topic %q", event.EventType, event.Topic), 0)
		return nil
	}
	return routed.handle(ctx, event)
}

func validRouterName(name string) bool {
	return name != "" && strings.TrimSpace(name) == name
}
