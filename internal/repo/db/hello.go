package db

// HelloRepository supplies greeting data to services.
type HelloRepository interface {
	Hello() string
}

// HelloRepo supplies a fixed greeting without external storage.
type HelloRepo struct{}

// NewHelloRepo creates the greeting repository.
func NewHelloRepo() *HelloRepo {
	return &HelloRepo{}
}

// Hello returns the demo greeting.
func (r *HelloRepo) Hello() string {
	return "hello"
}
