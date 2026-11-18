package event

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"goempty/pkg/async"
)

type testEvent struct {
	name   string
	userID int64
}

func (e testEvent) Name() string { return e.name }

type testHandler struct {
	name   string
	handle func(context.Context, Event) error
}

func (h testHandler) Name() string                              { return h.name }
func (h testHandler) Handle(ctx context.Context, e Event) error { return h.handle(ctx, e) }

func newTestBus(t *testing.T) (*Bus, *async.Pool) {
	t.Helper()
	p, err := async.New(async.Config{Concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := p.Stop(ctx); err != nil {
			t.Error(err)
		}
	})
	b, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	return b, p
}

func TestMultipleHandlersRunInOrderAndIsolateFailures(t *testing.T) {
	b, _ := newTestBus(t)
	var order []string
	done := make(chan struct{})
	for _, name := range []string{"first", "error", "panic", "last"} {
		if err := b.Subscribe("user.registered", testHandler{name: name, handle: func(ctx context.Context, e Event) error {
			if registered, ok := e.(testEvent); !ok || registered.userID != 42 || registered.name != "user.registered" {
				t.Errorf("event = %+v", e)
			}
			order = append(order, name)
			switch name {
			case "error":
				return errors.New("test handler failure")
			case "panic":
				panic("test handler panic")
			case "last":
				close(done)
			}
			return nil
		}}); err != nil {
			t.Fatal(err)
		}
	}
	b.Start()
	if err := b.Publish(context.Background(), testEvent{name: "user.registered", userID: 42}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handlers did not finish")
	}
	if !reflect.DeepEqual(order, []string{"first", "error", "panic", "last"}) {
		t.Fatalf("order = %v", order)
	}
}

func TestEventAdmissionAndRegistration(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("nil pool accepted")
	}
	b, p := newTestBus(t)
	if err := b.Publish(context.Background(), testEvent{name: "test"}); !errors.Is(err, ErrNotStarted) {
		t.Fatal(err)
	}
	release, entered := make(chan struct{}), make(chan struct{})
	defer close(release)
	handler := func(context.Context, Event) error { close(entered); <-release; return nil }
	if err := b.Subscribe("test", testHandler{name: "hold", handle: handler}); err != nil {
		t.Fatal(err)
	}
	if err := b.Subscribe("test", testHandler{name: "hold", handle: handler}); !errors.Is(err, ErrDuplicateHandler) {
		t.Fatal(err)
	}
	if err := b.Subscribe("", testHandler{name: "empty", handle: handler}); err == nil {
		t.Fatal("empty event name accepted")
	}
	if err := b.Subscribe("test", nil); err == nil {
		t.Fatal("nil handler accepted")
	}
	b.Start()
	if err := b.Subscribe("test", testHandler{name: "late", handle: handler}); !errors.Is(err, ErrRegistrationClosed) {
		t.Fatal(err)
	}
	if err := b.Publish(context.Background(), testEvent{name: "missing"}); !errors.Is(err, ErrNoHandlers) {
		t.Fatal(err)
	}
	if err := b.Publish(context.Background(), testEvent{name: "test"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	release <- struct{}{}
	if err := p.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := b.Publish(context.Background(), testEvent{name: "test"}); !errors.Is(err, async.ErrClosed) {
		t.Fatal(err)
	}
}
