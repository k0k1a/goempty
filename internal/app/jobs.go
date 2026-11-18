package app

import (
	"fmt"

	"goempty/internal/job"
	"goempty/internal/service"
	"goempty/pkg/scheduler"
)

type jobDependencies struct {
	HelloService *service.HelloService
}

func newScheduler(deps *jobDependencies) (*scheduler.Scheduler, error) {
	jobs, err := scheduler.New(scheduler.Config{})
	if err != nil {
		return nil, err
	}
	if err := jobs.Register(job.NewHelloJob(deps.HelloService)); err != nil {
		return nil, fmt.Errorf("register hello job: %w", err)
	}
	return jobs, nil
}
