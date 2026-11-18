package service

import "goempty/internal/repo/db"

// HelloService provides greeting operations.
type HelloService struct {
	helloRepo      db.HelloRepository
	helloPublisher HelloEventPublisher
}

// NewHelloService creates a service with its repository and event publisher.
func NewHelloService(helloRepo db.HelloRepository, helloPublisher HelloEventPublisher) *HelloService {
	return &HelloService{helloRepo: helloRepo, helloPublisher: helloPublisher}
}

// Hello returns the repository's greeting.
func (s *HelloService) Hello() string {
	return s.helloRepo.Hello()
}
