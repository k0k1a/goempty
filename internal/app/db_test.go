package app

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"goempty/config"
	"goempty/pkg/db"
	"goempty/pkg/httpserver"
	"goempty/pkg/mongodb"
	"goempty/pkg/redis"

	goredis "github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func testDatabaseConfig() db.Config {
	return db.Config{
		DSN:          "local@tcp(127.0.0.1:3306)/goempty?parseTime=true&timeout=1s&readTimeout=1s&writeTimeout=1s",
		MaxOpenConns: 2, MaxIdleConns: 1,
	}
}

func TestInitializeDatabaseUnavailable(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	// Accept and close the connection instead of completing a MySQL handshake.
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err == nil {
			conn.Close()
		}
	}()
	cfg := config.Config{
		MySQL: testDatabaseConfig(),
		HTTP:  httpserver.Config{Addr: "127.0.0.1:0"}, ShutdownTimeout: time.Second,
	}
	cfg.MySQL.DSN = "local@tcp(" + listener.Addr().String() + ")/goempty?parseTime=true&timeout=1s&readTimeout=1s&writeTimeout=1s"
	a, err := Initialize(context.Background(), cfg)
	if err == nil || a != nil {
		t.Fatal("failed MySQL handshake must fail application initialization")
	}
	listener.Close()
	<-done
}

func TestInitializeDatabaseValidation(t *testing.T) {
	cfg := config.Config{HTTP: httpserver.Config{Addr: "127.0.0.1:0"}, ShutdownTimeout: time.Second}
	a, err := Initialize(context.Background(), cfg)
	if err == nil || a != nil {
		t.Fatal("missing database configuration must fail without an App")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg.MySQL = testDatabaseConfig()
	a, err = Initialize(ctx, cfg)
	if !errors.Is(err, context.Canceled) || a != nil {
		t.Fatalf("canceled initialization: err=%v, has App=%v", err, a != nil)
	}
}

// This test requires a real database but does not create or change any tables.
func TestInitializeMySQLLifecycle(t *testing.T) {
	dsn := os.Getenv("GOEMPTY_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("GOEMPTY_TEST_MYSQL_DSN is not set")
	}
	redisAddr := os.Getenv("GOEMPTY_TEST_REDIS_ADDR")
	if redisAddr == "" {
		t.Skip("GOEMPTY_TEST_REDIS_ADDR is not set")
	}
	mongoURI := os.Getenv("GOEMPTY_TEST_MONGODB_URI")
	if mongoURI == "" {
		t.Skip("GOEMPTY_TEST_MONGODB_URI is not set; application requires MySQL, Redis and MongoDB")
	}
	cfg := config.Config{
		MySQL: testDatabaseConfig(),
		Redis: redis.Config{Addr: redisAddr,
			Username: os.Getenv("GOEMPTY_TEST_REDIS_USERNAME"), Password: os.Getenv("GOEMPTY_TEST_REDIS_PASSWORD")},
		MongoDB:         mongodb.Config{URI: mongoURI, ConnectTimeout: time.Second, ServerSelectionTimeout: 3 * time.Second},
		HTTP:            httpserver.Config{Addr: "127.0.0.1:0"},
		ShutdownTimeout: time.Second,
	}
	cfg.MySQL.DSN = dsn
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	a, err := Initialize(ctx, cfg)
	if err != nil {
		t.Fatal("MySQL application initialization failed")
	}
	t.Cleanup(a.Close)
	if err := db.Ping(ctx, a.database); err != nil {
		t.Fatal("assembled database is unavailable")
	}
	if err := redis.Ping(ctx, a.redisClient); err != nil {
		t.Fatal("assembled Redis is unavailable")
	}
	if err := mongodb.Ping(ctx, a.mongoClient); err != nil {
		t.Fatal("assembled MongoDB is unavailable")
	}
	// An already canceled Run must automatically release all three pools.
	canceled, stop := context.WithCancel(ctx)
	stop()
	if err := a.Run(canceled); err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(ctx, a.database); err == nil {
		t.Fatal("database remains open after application cleanup")
	}
	if err := a.redisClient.Ping(ctx).Err(); !errors.Is(err, goredis.ErrClosed) {
		t.Fatalf("Redis Ping after cleanup = %v, want ErrClosed", err)
	}
	if err := mongodb.Ping(ctx, a.mongoClient); !errors.Is(err, mongo.ErrClientDisconnected) {
		t.Fatal("MongoDB remains connected after application cleanup")
	}
	a.Close()
}
