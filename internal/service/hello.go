package service

// HelloService provides greeting operations.
type HelloService struct {
	repo HelloRepository
}

// NewHelloService creates a service with its repository dependency.
func NewHelloService(repo HelloRepository) *HelloService {
	return &HelloService{repo: repo}
}

// Hello returns the repository's greeting.
func (s *HelloService) Hello() string {
	return s.repo.Hello()
}
