package publisher

import (
	"context"
	"errors"
	"testing"

	"goempty/internal/entity"
	"goempty/pkg/kafka"
)

type publishCall struct {
	topic string
	event kafka.MessageRawData
}

type fakeProducer struct {
	calls []publishCall
	err   error
}

func (p *fakeProducer) Publish(_ context.Context, topic string, event kafka.MessageRawData, _ ...kafka.PublishOption) error {
	p.calls = append(p.calls, publishCall{topic: topic, event: event})
	return p.err
}

func (*fakeProducer) PublishAsync(context.Context, string, kafka.MessageRawData, func(error), ...kafka.PublishOption) error {
	panic("unused")
}

func (*fakeProducer) Close(context.Context) error { return nil }

func TestPublishGreeting(t *testing.T) {
	producer := &fakeProducer{}
	publisher := NewHelloPublisher(producer)
	message := entity.HelloGreetingMessage{Message: "hello"}
	if err := publisher.PublishGreeting(t.Context(), message); err != nil {
		t.Fatal(err)
	}
	if len(producer.calls) != 1 {
		t.Fatalf("publish calls = %d, want 1", len(producer.calls))
	}
	call := producer.calls[0]
	if call.topic != entity.HelloTopic || call.event.EventType != entity.HelloGreetingMQ {
		t.Fatalf("published topic/type = %q/%q", call.topic, call.event.EventType)
	}
	if call.event.Data != message {
		t.Fatalf("published data = %#v, want %#v", call.event.Data, message)
	}
	if call.event.EventID == "" || call.event.EventSource == "" || call.event.EventTime.IsZero() {
		t.Fatalf("message envelope = %#v", call.event)
	}
}

func TestPublishGreetingError(t *testing.T) {
	want := errors.New("broker unavailable")
	publisher := NewHelloPublisher(&fakeProducer{err: want})
	if err := publisher.PublishGreeting(t.Context(), entity.HelloGreetingMessage{Message: "hello"}); !errors.Is(err, want) {
		t.Fatalf("PublishGreeting = %v, want %v", err, want)
	}
}
