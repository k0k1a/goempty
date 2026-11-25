//go:build wireinject

package app

import (
	"context"

	"goempty/config"
	"goempty/internal/repo/db"
	"goempty/internal/repo/publisher"
	"goempty/internal/service"
	"goempty/pkg/async"
	"goempty/pkg/event"

	"github.com/google/wire"
)

// initialize assembles dependencies and returns cleanup for transfer to App.
func initialize(ctx context.Context, cfg config.Config) (*App, func(), error) {
	wire.Build(
		wire.FieldsOf(new(config.Config), "MySQL", "Postgres", "Redis", "MongoDB", "Kafka", "HTTP", "Async", "KafkaConsumer", "ShutdownTimeout"),
		async.New,
		event.New,
		newMySQL,
		newPostgres,
		newRedis,
		newMongoDB,
		newKafkaProducer,
		db.NewHelloRepo,
		publisher.NewHelloPublisher,
		wire.Bind(new(db.HelloRepository), new(*db.HelloRepo)),
		wire.Bind(new(service.HelloEventPublisher), new(*event.Bus)),
		wire.Bind(new(service.HelloMessagePublisher), new(*publisher.HelloPublisher)),
		service.NewHelloService,
		wire.Struct(new(httpDependencies), "*"),
		newHTTPServer,
		wire.Struct(new(jobDependencies), "*"),
		newScheduler,
		wire.Struct(new(eventDependencies), "*"),
		wire.Struct(new(messagingDependencies), "*"),
		newMessagingConsumers,
		newApp,
	)
	return nil, nil, nil
}
