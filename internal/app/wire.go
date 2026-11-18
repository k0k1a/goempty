//go:build wireinject

package app

import (
	"context"

	"goempty/config"
	"goempty/internal/repo/db"
	"goempty/internal/service"
	"goempty/pkg/async"
	"goempty/pkg/event"

	"github.com/google/wire"
)

// initialize assembles dependencies and returns cleanup for transfer to App.
func initialize(ctx context.Context, cfg config.Config) (*App, func(), error) {
	wire.Build(
		wire.FieldsOf(new(config.Config), "MySQL", "Redis", "MongoDB", "HTTP", "Async", "ShutdownTimeout"),
		async.New,
		event.New,
		newDatabase,
		newRedis,
		newMongoDB,
		db.NewHelloRepo,
		wire.Bind(new(db.HelloRepository), new(*db.HelloRepo)),
		wire.Bind(new(service.HelloEventPublisher), new(*event.Bus)),
		service.NewHelloService,
		wire.Struct(new(httpDependencies), "*"),
		newHTTPServer,
		wire.Struct(new(jobDependencies), "*"),
		newScheduler,
		wire.Struct(new(eventDependencies), "*"),
		newApp,
	)
	return nil, nil, nil
}
