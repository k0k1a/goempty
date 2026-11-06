package job

import (
	"context"
	"errors"
	"testing"

	"goempty/internal/service"
)

type helloRepo struct{ calls int }

func (r *helloRepo) Hello() string { r.calls++; return "job greeting" }

func TestHelloTask(t *testing.T) {
	repo := &helloRepo{}
	hello := service.NewHelloService(repo)
	jobs, err := NewScheduler(hello)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := jobs.Stop(context.Background()); err != nil {
			t.Error(err)
		}
	})
	if err := jobs.Register(helloTask(hello)); err == nil {
		t.Fatal("hello task was not registered")
	}
	task := helloTask(hello)
	if err := task.Handler(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repo.calls != 1 {
		t.Fatalf("service calls = %d", repo.calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := task.Handler(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled handler = %v", err)
	}
	if repo.calls != 1 {
		t.Fatal("canceled task called service")
	}
}
