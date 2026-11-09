package integration_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"goempty/pkg/mongodb"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Run against an isolated MongoDB deployment; no persistent data is written.
func TestMongoDBLifecycle(t *testing.T) {
	uri := os.Getenv("GOEMPTY_TEST_MONGODB_URI")
	if uri == "" {
		t.Skip("GOEMPTY_TEST_MONGODB_URI is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongodb.New(ctx, mongodb.Config{
		URI: uri, MaxPoolSize: 2, MinPoolSize: 1,
		ConnectTimeout: time.Second, ServerSelectionTimeout: 3 * time.Second,
		Timeout: 3 * time.Second,
	})
	if err != nil {
		t.Fatal("MongoDB initialization failed; check isolated test deployment settings")
	}
	closed := false
	t.Cleanup(func() {
		if closed {
			return
		}
		cleanupCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := mongodb.Close(cleanupCtx, client); err != nil {
			t.Error("MongoDB cleanup failed")
		}
	})
	if err := mongodb.Ping(ctx, client); err != nil {
		t.Fatal("MongoDB ping failed")
	}
	// Exercise a database command through the returned native client without writes.
	var result bson.M
	if err := client.Database("admin").RunCommand(ctx, bson.D{{Key: "ping", Value: 1}}).Decode(&result); err != nil {
		t.Fatal("MongoDB native command failed")
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if err := mongodb.Ping(canceled, client); !errors.Is(err, context.Canceled) {
		t.Fatal("Ping did not preserve context cancellation")
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := mongodb.Close(shutdownCtx, client); err != nil {
		t.Fatal("MongoDB shutdown failed")
	}
	closed = true
	if err := mongodb.Ping(ctx, client); !errors.Is(err, mongo.ErrClientDisconnected) {
		t.Fatal("Ping after Close did not return ErrClientDisconnected")
	}
}
