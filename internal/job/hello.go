package job

import (
	"context"
	"time"

	"goempty/internal/service"
	"goempty/pkg/log"
	"goempty/pkg/scheduler"

	"go.uber.org/zap"
)

func helloTask(helloService *service.HelloService) scheduler.Task {
	handler := &helloHandler{helloService: helloService}
	return scheduler.Task{Name: "hello", Spec: "* * * * *", Timeout: 5 * time.Second, Handler: handler.Handle}
}

type helloHandler struct{ helloService *service.HelloService }

func (h *helloHandler) Handle(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	log.Info(ctx, "Hello task greeting", zap.String("message", h.helloService.Hello()))
	return nil
}
