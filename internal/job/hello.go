// Package job contains the application's scheduled entry points.
package job

import (
	"context"
	"time"

	"goempty/internal/service"
)

// HelloJob schedules the greeting event demo.
type HelloJob struct {
	helloService *service.HelloService
}

func NewHelloJob(helloService *service.HelloService) *HelloJob {
	return &HelloJob{helloService: helloService}
}

func (*HelloJob) Name() string           { return "hello" }
func (*HelloJob) Spec() string           { return "* * * * *" }
func (*HelloJob) Timeout() time.Duration { return 5 * time.Second }

func (j *HelloJob) Run(ctx context.Context) error {
	return j.helloService.PublishGreeting(ctx)
}
