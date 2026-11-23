package service

import (
	"context"

	"goempty/internal/entity"
)

// HelloMessagePublisher publishes hello messages to Kafka.
type HelloMessagePublisher interface {
	PublishGreeting(ctx context.Context, message entity.HelloGreetingMessage) error
}

// PublishGreetingMessage publishes a greeting message to Kafka.
func (s *HelloService) PublishGreetingMessage(ctx context.Context, message entity.HelloGreetingMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.helloMessagePublisher.PublishGreeting(ctx, message)
}
