package integration_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"goempty/pkg/db"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Run against an isolated MySQL database with permission to create temporary tables.
func TestMySQLLifecycleAndTransactions(t *testing.T) {
	dsn := os.Getenv("GOEMPTY_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("GOEMPTY_TEST_MYSQL_DSN is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	handle, err := db.New(ctx, db.Config{
		DSN:          dsn,
		MaxOpenConns: 2, MaxIdleConns: 1,
		ConnMaxLifetime: time.Minute, ConnMaxIdleTime: time.Minute,
	})
	if err != nil {
		t.Fatal("MySQL initialization failed; check test database connectivity and configuration")
	}
	handle = db.SetLogger(handle, logger.Discard)
	t.Cleanup(func() {
		if err := db.Close(handle); err != nil {
			t.Error(err)
		}
	})
	if err := db.Ping(ctx, handle); err != nil {
		t.Fatal(err)
	}
	err = handle.WithContext(ctx).Connection(func(conn *gorm.DB) error {
		if err := conn.Exec("CREATE TEMPORARY TABLE goempty_db_test (id BIGINT PRIMARY KEY) ENGINE=InnoDB").Error; err != nil {
			return err
		}
		// The temporary table is scoped to this connection and disappears on close.
		if err := conn.Transaction(func(tx *gorm.DB) error {
			return tx.Exec("INSERT INTO goempty_db_test (id) VALUES (?)", 1).Error
		}); err != nil {
			return err
		}
		rollback := errors.New("rollback requested")
		if err := conn.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("INSERT INTO goempty_db_test (id) VALUES (?)", 2).Error; err != nil {
				return err
			}
			return rollback
		}); !errors.Is(err, rollback) {
			return errors.New("transaction did not return rollback error")
		}
		var ids []int64
		if err := conn.Table("goempty_db_test").Order("id").Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) != 1 || ids[0] != 1 {
			return errors.New("commit or rollback did not take effect")
		}
		canceled, stop := context.WithCancel(ctx)
		stop()
		if err := conn.WithContext(canceled).Exec("SELECT 1").Error; !errors.Is(err, context.Canceled) {
			return errors.New("query did not honor cancellation")
		}
		return conn.Exec("DROP TEMPORARY TABLE goempty_db_test").Error
	})
	if err != nil {
		t.Fatal(err)
	}
}
