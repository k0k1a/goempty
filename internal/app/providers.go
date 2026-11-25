package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"goempty/config"
	"goempty/pkg/async"
	"goempty/pkg/db"
	"goempty/pkg/event"
	"goempty/pkg/httpserver"
	"goempty/pkg/kafka"
	"goempty/pkg/log"
	"goempty/pkg/mongodb"
	"goempty/pkg/postgres"
	"goempty/pkg/redis"
	"goempty/pkg/scheduler"

	goredis "github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Initialize assembles the application and transfers dependency cleanup to App.
func Initialize(ctx context.Context, cfg config.Config) (*App, error) {
	a, cleanup, err := initialize(ctx, cfg)
	if err != nil {
		return nil, err
	}
	a.cleanup = cleanup
	return a, nil
}

// mysqlHandle and postgresHandle label the relational database pools so Wire
// can keep two *gorm.DB providers apart.
type mysqlHandle struct{ *gorm.DB }

type postgresHandle struct{ *gorm.DB }

// newMySQL owns the shared pool. Wire cleans it up on assembly failure;
// App closes it after HTTP and task shutdown on successful initialization.
func newMySQL(ctx context.Context, cfg db.Config) (mysqlHandle, func(), error) {
	handle, err := db.New(ctx, cfg)
	if err != nil {
		return mysqlHandle{}, nil, err
	}
	return mysqlHandle{handle}, func() {
		if err := db.Close(handle); err != nil {
			log.Error(context.WithoutCancel(ctx), "Close MySQL failed", zap.Error(err))
		}
	}, nil
}

// newPostgres owns the shared pool. Wire cleans it up on assembly failure;
// App closes it after HTTP and task shutdown on successful initialization.
func newPostgres(ctx context.Context, cfg postgres.Config) (postgresHandle, func(), error) {
	handle, err := postgres.New(ctx, cfg)
	if err != nil {
		return postgresHandle{}, nil, err
	}
	return postgresHandle{handle}, func() {
		if err := postgres.Close(handle); err != nil {
			log.Error(context.WithoutCancel(ctx), "Close PostgreSQL failed", zap.Error(err))
		}
	}, nil
}

// newRedis owns the shared pool. Wire releases it on assembly failure;
// App runs cleanup after HTTP and task shutdown on successful initialization.
func newRedis(ctx context.Context, cfg redis.Config) (*goredis.Client, func(), error) {
	client, err := redis.New(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	return client, func() {
		if err := redis.Close(client); err != nil {
			log.Error(context.WithoutCancel(ctx), "Close Redis failed", zap.Error(err))
		}
	}, nil
}

// newMongoDB owns the shared client. Wire cleans it up on assembly failure;
// App releases it after HTTP and task shutdown, even if startup was canceled.
func newMongoDB(ctx context.Context, cfg mongodb.Config, shutdownTimeout time.Duration) (*mongo.Client, func(), error) {
	if shutdownTimeout <= 0 {
		return nil, nil, errors.New("app: shutdown timeout must be positive")
	}
	client, err := mongodb.New(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	return client, func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()
		if err := mongodb.Close(cleanupCtx, client); err != nil {
			log.Error(cleanupCtx, "Close MongoDB failed", zap.Error(err))
		}
	}, nil
}

// newKafkaProducer creates the shared publisher. Wire releases it on assembly
// failure; App runs cleanup after components stop, once publishing is over.
// Close flushes buffered deliveries within its own delivery timeout.
func newKafkaProducer(ctx context.Context, cfg kafka.Config) (kafka.Producer, func(), error) {
	producer, err := kafka.NewProducer(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	return producer, func() {
		closeCtx := context.WithoutCancel(ctx)
		if err := producer.Close(closeCtx); err != nil {
			log.Error(closeCtx, "Close Kafka producer failed", zap.Error(err))
		}
	}, nil
}

// App retains the shared pools for its lifetime; business consumers receive
// them directly through their constructors when storage-backed code is added.
func newApp(server *httpserver.Server, jobs *scheduler.Scheduler, taskPool *async.Pool, eventBus *event.Bus, consumers []*kafka.Consumer, eventDeps *eventDependencies, shutdownTimeout time.Duration, mysql mysqlHandle, postgres postgresHandle, client *goredis.Client, mongoClient *mongo.Client) (*App, error) {
	a, err := New(server, jobs, taskPool, eventBus, consumers, shutdownTimeout)
	if err != nil {
		return nil, err
	}
	if err := registerEvents(eventBus, eventDeps); err != nil {
		return nil, fmt.Errorf("register event controllers: %w", err)
	}
	a.mysql = mysql.DB
	a.postgres = postgres.DB
	a.redisClient = client
	a.mongoClient = mongoClient
	return a, nil
}
