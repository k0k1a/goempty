package app

import (
	"context"
	"errors"
	"time"

	"goempty/config"
	"goempty/pkg/db"
	"goempty/pkg/httpserver"
	"goempty/pkg/log"
	"goempty/pkg/mongodb"
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

// newDatabase owns the shared pool. Wire cleans it up on assembly failure;
// App closes it after HTTP and task shutdown on successful initialization.
func newDatabase(ctx context.Context, cfg db.Config) (*gorm.DB, func(), error) {
	handle, err := db.New(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	return handle, func() {
		if err := db.Close(handle); err != nil {
			log.Error(context.WithoutCancel(ctx), "Close database failed", zap.Error(err))
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

// App retains the shared pools for its lifetime; business consumers receive
// them directly through their constructors when storage-backed code is added.
func newApp(server *httpserver.Server, jobs *scheduler.Scheduler, shutdownTimeout time.Duration, handle *gorm.DB, client *goredis.Client, mongoClient *mongo.Client) (*App, error) {
	a, err := New(server, jobs, shutdownTimeout)
	if err != nil {
		return nil, err
	}
	a.database = handle
	a.redisClient = client
	a.mongoClient = mongoClient
	return a, nil
}
