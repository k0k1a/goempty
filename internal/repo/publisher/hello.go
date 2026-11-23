// Package publisher contains the application's Kafka publish implementations.
package publisher

import (
	"context"

	"goempty/internal/entity"
	"goempty/pkg/kafka"
)

// HelloPublisher publishes hello messages to Kafka.
type HelloPublisher struct {
	producer kafka.Producer
}

// NewHelloPublisher creates a publisher backed by the shared Kafka producer.
func NewHelloPublisher(producer kafka.Producer) *HelloPublisher {
	return &HelloPublisher{producer: producer}
}

// PublishGreeting publishes one HelloGreetingMQ message on the hello topic.
func (p *HelloPublisher) PublishGreeting(ctx context.Context, message entity.HelloGreetingMessage) error {
	return p.producer.PublishSync(ctx, entity.HelloTopic, kafka.NewMessageRawData(entity.HelloGreetingMQ, message))
}
