package mongodb

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestNilClient(t *testing.T) {
	ctx := context.Background()
	if err := Ping(ctx, nil); err == nil {
		t.Fatal("Ping(nil) succeeded")
	}
	if err := Close(ctx, nil); err == nil {
		t.Fatal("Close(nil) succeeded")
	}
}

func TestCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client, err := New(ctx, Config{URI: "mongodb://localhost"})
	if client != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("New() = %v, %v; want nil, context.Canceled", client, err)
	}
	// Only the owned test endpoint can receive background monitoring connections.
	client, err = mongo.Connect(options.Client().ApplyURI(rejectingURI(t)))
	if err != nil {
		t.Fatal(err)
	}
	closeCtx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if err := Close(closeCtx, client); err != nil {
		t.Fatal(err)
	}
	if err := Ping(ctx, client); !errors.Is(err, context.Canceled) {
		t.Fatalf("Ping() = %v, want context.Canceled", err)
	}
	if err := Ping(closeCtx, client); !errors.Is(err, mongo.ErrClientDisconnected) {
		t.Fatalf("Ping() = %v, want ErrClientDisconnected", err)
	}
}

func TestNewInvalidConfig(t *testing.T) {
	client, err := New(context.Background(), Config{})
	if err == nil || client != nil {
		t.Fatalf("New() = %v, %v; want nil, error", client, err)
	}
}

func TestNewFailedPing(t *testing.T) {
	uri := rejectingURI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	client, err := New(ctx, Config{URI: uri})
	if client != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("New() = %v, %v; want nil, context.DeadlineExceeded", client, err)
	}
}

func rejectingURI(t *testing.T) string {
	t.Helper()
	// Own the endpoint so this never connects to a real MongoDB service.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close() // Test endpoint intentionally rejects every handshake.
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close() // Unblocks Accept before joining the endpoint goroutine.
		<-done
	})
	return "mongodb://" + listener.Addr().String() + "/?directConnection=true"
}
