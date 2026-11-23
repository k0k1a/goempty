package job

import (
	"context"
	"errors"
	"testing"

	"goempty/internal/entity"
	"goempty/internal/service"
	"goempty/pkg/event"
)

type helloRepo struct{ calls int }

func (r *helloRepo) Hello() string {
	r.calls++
	return "job greeting"
}

type greetingPublisher struct {
	events []event.Event
	err    error
}

func (p *greetingPublisher) Publish(_ context.Context, e event.Event) error {
	p.events = append(p.events, e)
	return p.err
}

type greetingMessagePublisher struct{}

func (*greetingMessagePublisher) PublishGreeting(context.Context, entity.HelloGreetingMessage) error {
	panic("Hello Job must use the in-process event publisher")
}

func TestHelloJobPublishesGreeting(t *testing.T) {
	repo := &helloRepo{}
	publisher := &greetingPublisher{}
	job := NewHelloJob(service.NewHelloService(repo, publisher, &greetingMessagePublisher{}))
	if err := job.Run(t.Context()); err != nil {
		t.Fatal(err)
	}
	want := entity.HelloGreetingEvent{Message: "job greeting"}
	if repo.calls != 1 || len(publisher.events) != 1 || publisher.events[0] != want {
		t.Fatalf("service calls = %d, events = %#v", repo.calls, publisher.events)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := job.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled job = %v", err)
	}
	if repo.calls != 1 || len(publisher.events) != 1 {
		t.Fatal("canceled job read or published a greeting")
	}

	publisher.err = errors.New("event submission failed")
	if err := job.Run(t.Context()); !errors.Is(err, publisher.err) {
		t.Fatalf("job error = %v, want %v", err, publisher.err)
	}
}
