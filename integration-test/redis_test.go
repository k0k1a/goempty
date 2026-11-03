package integration_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"goempty/pkg/redis"
)

// Run against an isolated Redis instance; no persistent data is written.
func TestRedisLifecycle(t *testing.T) {
	addr := os.Getenv("GOEMPTY_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("GOEMPTY_TEST_REDIS_ADDR is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := redis.New(ctx, redis.Config{
		Addr: addr, Username: os.Getenv("GOEMPTY_TEST_REDIS_USERNAME"),
		Password: os.Getenv("GOEMPTY_TEST_REDIS_PASSWORD"),
		PoolSize: 2, MinIdleConns: 1,
		DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second,
	})
	if err != nil {
		t.Fatal("Redis initialization failed; check test server connectivity and credentials")
	}
	t.Cleanup(func() {
		if err := redis.Close(client); err != nil {
			t.Error(err)
		}
	})
	if err := redis.Ping(ctx, client); err != nil {
		t.Fatal(err)
	}
	// ECHO exercises the returned native client without modifying shared data.
	if result, err := client.Echo(ctx, "goempty").Result(); err != nil || result != "goempty" {
		t.Fatalf("Echo() = %q, %v", result, err)
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if err := redis.Ping(canceled, client); !errors.Is(err, context.Canceled) {
		t.Fatalf("Ping() = %v, want context.Canceled", err)
	}
	if err := client.Ping(canceled).Err(); !errors.Is(err, context.Canceled) {
		t.Fatalf("native Ping() = %v, want context.Canceled", err)
	}
	// Use a separate client to verify shutdown without closing the cleanup client twice.
	other, err := redis.New(ctx, redis.Config{Addr: addr,
		Username: os.Getenv("GOEMPTY_TEST_REDIS_USERNAME"), Password: os.Getenv("GOEMPTY_TEST_REDIS_PASSWORD")})
	if err != nil {
		t.Fatal("Redis initialization failed")
	}
	if err := redis.Close(other); err != nil {
		t.Fatal(err)
	}
	if err := other.Ping(ctx).Err(); !errors.Is(err, goredis.ErrClosed) {
		t.Fatalf("Ping after Close = %v, want ErrClosed", err)
	}
}
