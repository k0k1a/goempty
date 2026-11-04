package service

import "goempty/internal/repo/db"

// HelloService provides greeting operations.
type HelloService struct {
	repo db.HelloRepository
}

// NewHelloService creates a service with its repository dependency.
func NewHelloService(repo db.HelloRepository) *HelloService {
	return &HelloService{repo: repo}
}

// Hello returns the repository's greeting.
func (s *HelloService) Hello() string {
	return s.repo.Hello()
}
