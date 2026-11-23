// Package consumer contains the application's Kafka message entry points.
package consumer

import (
	"context"
	"errors"

	"goempty/internal/entity"
	"goempty/internal/service"
	"goempty/pkg/kafka"
)

// HelloGreetingConsumer logs hello greeting messages consumed from Kafka.
type HelloGreetingConsumer struct {
	helloService *service.HelloService
}

// NewHelloGreetingConsumer creates a consumer entry bound to the Hello service.
func NewHelloGreetingConsumer(helloService *service.HelloService) *HelloGreetingConsumer {
	return &HelloGreetingConsumer{helloService: helloService}
}

func (*HelloGreetingConsumer) Name() string { return "helloGreeting" }

func (c *HelloGreetingConsumer) Handle(ctx context.Context, _ kafka.MessageRawData, data entity.HelloGreetingMessage) error {
	if data.Message == "" {
		return errors.New("invalid hello greeting message")
	}
	return c.helloService.LogGreeting(ctx, entity.HelloGreetingEvent{Message: data.Message})
}
