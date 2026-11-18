package service

import (
	"context"

	"goempty/internal/entity"
	"goempty/pkg/event"
	"goempty/pkg/log"

	"go.uber.org/zap"
)

// HelloEventPublisher submits process-local events for the Hello service.
type HelloEventPublisher interface {
	Publish(context.Context, event.Event) error
}

// PublishGreeting reads a greeting and submits it to the event bus.
func (s *HelloService) PublishGreeting(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.helloPublisher.Publish(ctx, entity.HelloGreetingEvent{Message: s.Hello()})
}

// LogGreeting handles the greeting without writing to storage.
func (s *HelloService) LogGreeting(ctx context.Context, greeting entity.HelloGreetingEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	log.Info(ctx, "Hello task greeting", zap.String("message", greeting.Message))
	return nil
}
