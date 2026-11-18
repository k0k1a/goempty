// Package event contains the application's process-local event entry points.
package event

import (
	"context"
	"errors"

	"goempty/internal/entity"
	"goempty/internal/service"
	"goempty/pkg/event"
)

// HelloEventController forwards greeting events to the Hello service.
type HelloEventController struct {
	helloService *service.HelloService
}

func NewHelloEventController(helloService *service.HelloService) *HelloEventController {
	return &HelloEventController{helloService: helloService}
}

func (*HelloEventController) Name() string { return "helloGreeting" }

func (c *HelloEventController) Handle(ctx context.Context, e event.Event) error {
	greeting, ok := e.(entity.HelloGreetingEvent)
	if !ok || greeting.Message == "" {
		return errors.New("invalid hello greeting event")
	}
	return c.helloService.LogGreeting(ctx, greeting)
}
