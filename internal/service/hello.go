package service

import "goempty/internal/repo/db"

// HelloService provides greeting operations.
type HelloService struct {
	helloRepo db.HelloRepository
}

// NewHelloService creates a service with its repository dependency.
func NewHelloService(helloRepo db.HelloRepository) *HelloService {
	return &HelloService{helloRepo: helloRepo}
}

// Hello returns the repository's greeting.
func (s *HelloService) Hello() string {
	return s.helloRepo.Hello()
}
