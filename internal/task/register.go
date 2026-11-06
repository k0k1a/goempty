// Package task assembles business task registrations before the application starts.
package task

import "goempty/pkg/scheduler"

// NewScheduler is the application's task registration entry point. Business
// services should be constructor parameters as tasks are added to registrations.
// There are currently no scheduled business operations in this application.
func NewScheduler() (*scheduler.Scheduler, error) {
	s, err := scheduler.New(scheduler.Config{})
	if err != nil {
		return nil, err
	}
	registrations := []scheduler.Task{}
	for _, task := range registrations {
		if err := s.Register(task); err != nil {
			return nil, err
		}
	}
	return s, nil
}
