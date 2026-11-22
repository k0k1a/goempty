package app

import (
	"context"
	"testing"
	"time"

	"goempty/config"
	"goempty/pkg/httpserver"
	"goempty/pkg/kafka"
)

func TestAppManagesKafkaConsumers(t *testing.T) {
	consumer, err := kafka.NewConsumer(
		kafka.ConsumerConfig{
			Brokers:      []string{"127.0.0.1:1"},
			GroupName:    "forum",
			Topics:       []string{"social-forum"},
			TopicMapping: map[string]string{"social-forum": "forum-test"},
		},
		func(context.Context, kafka.MessageRawData) error { return nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	a, err := newHTTPTestApp(config.Config{
		HTTP: httpserver.Config{Addr: "127.0.0.1:0"}, ShutdownTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	a.consumers = []*kafka.Consumer{consumer}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()
	deadline := time.After(time.Second)
	for consumer.Done() == nil {
		select {
		case <-deadline:
			t.Fatal("app did not start the consumer")
		case <-time.After(time.Millisecond):
		}
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop")
	}
	select {
	case <-consumer.Done():
	default:
		t.Fatal("app did not stop the consumer")
	}
}
