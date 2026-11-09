package app

import (
	"context"
	"errors"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"goempty/pkg/mongodb"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

func TestNewMongoDBValidation(t *testing.T) {
	client, cleanup, err := newMongoDB(context.Background(), mongodb.Config{}, time.Second)
	if err == nil || client != nil || cleanup != nil {
		t.Fatal("missing MongoDB configuration must fail without a client or cleanup")
	}
	client, cleanup, err = newMongoDB(context.Background(), mongodb.Config{}, 0)
	if err == nil || !strings.Contains(err.Error(), "shutdown timeout") || client != nil || cleanup != nil {
		t.Fatal("invalid shutdown timeout must fail before connecting")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client, cleanup, err = newMongoDB(ctx, mongodb.Config{URI: "mongodb://127.0.0.1:27017"}, time.Second)
	if !errors.Is(err, context.Canceled) || client != nil || cleanup != nil {
		t.Fatal("canceled initialization must fail without a client or cleanup")
	}
}

func TestNewMongoDBUnavailable(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	// The owned endpoint never answers a MongoDB handshake.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	client, cleanup, err := newMongoDB(ctx, mongodb.Config{
		URI: "mongodb://" + listener.Addr().String() + "/?directConnection=true",
	}, time.Second)
	if !errors.Is(err, context.DeadlineExceeded) || client != nil || cleanup != nil {
		t.Fatal("unavailable MongoDB must fail without a client or cleanup")
	}
}

// Uses an isolated deployment without writing persistent data.
func TestNewMongoDBLifecycle(t *testing.T) {
	uri := os.Getenv("GOEMPTY_TEST_MONGODB_URI")
	if uri == "" {
		t.Skip("GOEMPTY_TEST_MONGODB_URI is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, cleanup, err := newMongoDB(ctx, mongodb.Config{URI: uri}, time.Second)
	if err != nil {
		t.Fatal("MongoDB initialization failed; check isolated test deployment settings")
	}
	// Cleanup must release the pool even when the startup context is canceled.
	cancel()
	cleanup()
	checkCtx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if err := mongodb.Ping(checkCtx, client); !errors.Is(err, mongo.ErrClientDisconnected) {
		t.Fatal("MongoDB remains connected after provider cleanup")
	}
}
