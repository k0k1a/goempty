package app

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"goempty/pkg/redis"

	goredis "github.com/redis/go-redis/v9"
)

func TestNewRedisValidation(t *testing.T) {
	client, cleanup, err := newRedis(context.Background(), redis.Config{})
	if err == nil || client != nil || cleanup != nil {
		t.Fatal("missing Redis configuration must fail without a client or cleanup")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client, cleanup, err = newRedis(ctx, redis.Config{Addr: "127.0.0.1:6379"})
	if !errors.Is(err, context.Canceled) || client != nil || cleanup != nil {
		t.Fatal("canceled initialization must fail without a client or cleanup")
	}
}

func TestNewRedisUnavailable(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	// Leave the connection unanswered to exercise the startup PING deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	client, cleanup, err := newRedis(ctx, redis.Config{
		Addr: listener.Addr().String(), DialTimeout: 100 * time.Millisecond,
		ReadTimeout: 100 * time.Millisecond, WriteTimeout: 100 * time.Millisecond,
	})
	if err == nil || client != nil || cleanup != nil {
		t.Fatal("unavailable Redis must fail without a client or cleanup")
	}
}

func TestNewRedisLifecycle(t *testing.T) {
	addr := os.Getenv("GOEMPTY_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("GOEMPTY_TEST_REDIS_ADDR is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, cleanup, err := newRedis(ctx, redis.Config{Addr: addr,
		Username: os.Getenv("GOEMPTY_TEST_REDIS_USERNAME"), Password: os.Getenv("GOEMPTY_TEST_REDIS_PASSWORD")})
	if err != nil {
		t.Fatal("Redis initialization failed; check test server connectivity and credentials")
	}
	// Cleanup must also work after the startup context is canceled.
	cancel()
	cleanup()
	if err := client.Ping(context.Background()).Err(); !errors.Is(err, goredis.ErrClosed) {
		t.Fatalf("Ping after cleanup = %v, want ErrClosed", err)
	}
}
