// Package kafka publishes and consumes application events using franz-go.
package kafka

import (
	"errors"
	"strings"
	"time"
)

// Config contains producer connection settings.
type Config struct {
	Brokers      []string          `mapstructure:"brokers"`
	TopicMapping map[string]string `mapstructure:"topic_mapping"`
}

// ConsumerConfig contains one consumer group's connection and subscription
// settings. GroupName is used verbatim as the Kafka group ID.
type ConsumerConfig struct {
	Brokers      []string          `mapstructure:"brokers"`
	GroupName    string            `mapstructure:"group_name"`
	Topics       []string          `mapstructure:"topics"`
	TopicMapping map[string]string `mapstructure:"topic_mapping"`
	// MaxAttempts bounds handler attempts for one record. Zero uses the
	// default of 3; a negative value is rejected.
	MaxAttempts int `mapstructure:"max_attempts"`
	// RetryBackoff is the wait before the second handler attempt. Later
	// attempts double it up to 30s. Zero uses the default of 1s.
	RetryBackoff time.Duration `mapstructure:"retry_backoff"`
}

func (c Config) brokers() ([]string, error) {
	return validateBrokers(c.Brokers)
}

func (c ConsumerConfig) brokers() ([]string, error) {
	return validateBrokers(c.Brokers)
}

func validateBrokers(brokers []string) ([]string, error) {
	if len(brokers) == 0 {
		return nil, errors.New("kafka: brokers are required")
	}
	for _, broker := range brokers {
		if strings.TrimSpace(broker) == "" {
			return nil, errors.New("kafka: broker address is empty")
		}
	}
	return brokers, nil
}
