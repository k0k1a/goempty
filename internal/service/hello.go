package service

import "goempty/internal/repo/db"

// HelloService provides greeting operations.
type HelloService struct {
	helloRepo             db.HelloRepository
	helloPublisher        HelloEventPublisher
	helloMessagePublisher HelloMessagePublisher
}

// NewHelloService creates a service with its repository and publishers.
func NewHelloService(helloRepo db.HelloRepository, helloPublisher HelloEventPublisher, helloMessagePublisher HelloMessagePublisher) *HelloService {
	return &HelloService{
		helloRepo:             helloRepo,
		helloPublisher:        helloPublisher,
		helloMessagePublisher: helloMessagePublisher,
	}
}

// Hello returns the repository's greeting.
func (s *HelloService) Hello() string {
	return s.helloRepo.Hello()
}
