package redis

import (
	"context"
	"errors"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
)

// New creates a shared client and verifies connectivity with PING.
// The caller owns the client and must call Close at application shutdown.
func New(ctx context.Context, cfg Config) (*goredis.Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("redis: initialize: %w", err)
	}
	client := goredis.NewClient(&goredis.Options{
		Addr:                  cfg.Addr,
		Username:              cfg.Username,
		Password:              cfg.Password,
		DB:                    cfg.DB,
		PoolSize:              cfg.PoolSize,
		MinIdleConns:          cfg.MinIdleConns,
		DialTimeout:           cfg.DialTimeout,
		ReadTimeout:           cfg.ReadTimeout,
		WriteTimeout:          cfg.WriteTimeout,
		PoolTimeout:           cfg.PoolTimeout,
		ContextTimeoutEnabled: true,
	})
	if err := Ping(ctx, client); err != nil {
		return nil, errors.Join(err, Close(client))
	}
	return client, nil
}

// Ping checks connectivity using the caller's context and socket timeouts.
func Ping(ctx context.Context, client *goredis.Client) error {
	if client == nil {
		return errors.New("redis: client is required")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("redis: ping: %w", err)
	}
	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis: ping: %w", err)
	}
	return nil
}

// Close releases the shared connection pool. Call it once at app shutdown.
func Close(client *goredis.Client) error {
	if client == nil {
		return errors.New("redis: client is required")
	}
	if err := client.Close(); err != nil {
		return fmt.Errorf("redis: close: %w", err)
	}
	return nil
}
