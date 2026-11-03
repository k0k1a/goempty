package service

// HelloRepository supplies the greeting used by HelloService.
type HelloRepository interface {
	Hello() string
}
