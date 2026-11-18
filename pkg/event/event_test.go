package event

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"goempty/pkg/async"
)

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
		if err := b.Register("user.registered", name, func(ctx context.Context, e Event) error {
			if e.Payload != int64(42) || e.Name != "user.registered" {
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
		}); err != nil {
			t.Fatal(err)
		}
	}
	b.Start()
	if err := b.Publish(context.Background(), Event{Name: "user.registered", Payload: int64(42)}); err != nil {
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
	if err := b.Publish(context.Background(), Event{Name: "test"}); !errors.Is(err, ErrNotStarted) {
		t.Fatal(err)
	}
	release, entered := make(chan struct{}), make(chan struct{})
	defer close(release)
	handler := func(context.Context, Event) error { close(entered); <-release; return nil }
	if err := b.Register("test", "hold", handler); err != nil {
		t.Fatal(err)
	}
	if err := b.Register("test", "hold", handler); !errors.Is(err, ErrDuplicateHandler) {
		t.Fatal(err)
	}
	if err := b.Register("", "empty", handler); err == nil {
		t.Fatal("empty event name accepted")
	}
	if err := b.Register("test", "nil", nil); err == nil {
		t.Fatal("nil handler accepted")
	}
	b.Start()
	if err := b.Register("test", "late", handler); !errors.Is(err, ErrRegistrationClosed) {
		t.Fatal(err)
	}
	if err := b.Publish(context.Background(), Event{Name: "missing"}); !errors.Is(err, ErrNoHandlers) {
		t.Fatal(err)
	}
	if err := b.Publish(context.Background(), Event{Name: "test"}); err != nil {
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
	if err := b.Publish(context.Background(), Event{Name: "test"}); !errors.Is(err, async.ErrClosed) {
		t.Fatal(err)
	}
}
