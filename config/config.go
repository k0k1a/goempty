// Package config loads application configuration.
package config

import (
	"sync"
	"time"

	"goempty/pkg/async"
	"goempty/pkg/db"
	"goempty/pkg/httpserver"
	"goempty/pkg/kafka"
	"goempty/pkg/mongodb"
	"goempty/pkg/postgres"
	"goempty/pkg/redis"
)

// Config contains file settings and startup settings supplied by the entry point.
type Config struct {
	Async           async.Config         `mapstructure:"async"`
	MySQL           db.Config            `mapstructure:"mysql"`
	Postgres        postgres.Config      `mapstructure:"postgres"`
	Redis           redis.Config         `mapstructure:"redis"`
	MongoDB         mongodb.Config       `mapstructure:"mongodb"`
	Kafka           kafka.Config         `mapstructure:"kafka"`
	KafkaConsumer   kafka.ConsumerConfig `mapstructure:"kafka_consumer"`
	HTTP            httpserver.Config    `mapstructure:"-"`
	ShutdownTimeout time.Duration        `mapstructure:"-"`
}

var (
	mu     sync.RWMutex
	config Config
)

// Get returns the latest successfully decoded configuration snapshot.
// Call Load once during startup before reading configuration.
// HTTP and ShutdownTimeout remain zero here; main sets them on its startup copy.
func Get() Config {
	mu.RLock()
	defer mu.RUnlock()
	return config
}

func set(cfg Config) {
	mu.Lock()
	defer mu.Unlock()
	config = cfg
}
