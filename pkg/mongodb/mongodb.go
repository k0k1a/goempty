package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

const cleanupTimeout = 5 * time.Second

// New creates a shared client and verifies connectivity using its read preference.
// The caller owns the client and must call Close at application shutdown.
// Supply a deadline to bound initialization, including its connectivity check.
func New(ctx context.Context, cfg Config) (*mongo.Client, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("mongodb: initialize: %w", err)
	}
	opts, err := cfg.clientOptions()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("mongodb: initialize: %w", err)
	}
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, errors.New("mongodb: client initialization failed; check connection settings")
	}
	if err := Ping(ctx, client); err != nil {
		// Cleanup must still run when initialization's context has expired.
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		defer cancel()
		return nil, errors.Join(err, Close(cleanupCtx, client))
	}
	return client, nil
}

// Ping checks connectivity with the caller's context and the client's read preference.
func Ping(ctx context.Context, client *mongo.Client) error {
	if client == nil {
		return errors.New("mongodb: client is required")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("mongodb: ping: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("mongodb: ping: %w", err)
	}
	return nil
}

// Close disconnects the client and releases its pool. Call it once at shutdown,
// after in-flight operations stop, using a context with a fresh shutdown deadline.
// Even if ctx expires, the driver closes connections; unfinished operations may fail.
func Close(ctx context.Context, client *mongo.Client) error {
	if client == nil {
		return errors.New("mongodb: client is required")
	}
	if err := client.Disconnect(ctx); err != nil {
		return fmt.Errorf("mongodb: close: %w", err)
	}
	return nil
}
