// Package job assembles business task registrations before the application starts.
package job

import (
	"goempty/internal/service"
	"goempty/pkg/scheduler"
)

// NewScheduler is the application's task registration entry point. Business
// services should be constructor parameters as tasks are added to registrations.
func NewScheduler(hello *service.HelloService) (*scheduler.Scheduler, error) {
	s, err := scheduler.New(scheduler.Config{})
	if err != nil {
		return nil, err
	}
	registrations := []scheduler.Task{helloTask(hello)}
	for _, task := range registrations {
		if err := s.Register(task); err != nil {
			return nil, err
		}
	}
	return s, nil
}
