package app

import (
	"goempty/internal/entity"
	evententry "goempty/internal/event"
	"goempty/internal/service"
	"goempty/pkg/event"
)

type eventDependencies struct {
	HelloService *service.HelloService
}

func registerEvents(bus *event.Bus, deps *eventDependencies) error {
	return bus.Subscribe(entity.HelloGreetingRequested, evententry.NewHelloEventController(deps.HelloService))
}
