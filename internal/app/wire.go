//go:build wireinject

package app

import (
	"context"

	"goempty/config"
	"goempty/internal/repo/db"
	"goempty/internal/service"

	"github.com/google/wire"
)

// initialize assembles dependencies and returns cleanup for transfer to App.
func initialize(ctx context.Context, cfg config.Config) (*App, func(), error) {
	wire.Build(
		wire.FieldsOf(new(config.Config), "MySQL", "Redis", "HTTP", "ShutdownTimeout"),
		newDatabase,
		newRedis,
		db.NewHelloRepo,
		wire.Bind(new(service.HelloRepository), new(*db.HelloRepo)),
		service.NewHelloService,
		wire.Struct(new(httpDependencies), "*"),
		newHTTPServer,
		newApp,
	)
	return nil, nil, nil
}
