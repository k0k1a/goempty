package config

import (
	"slices"
	"testing"
	"time"
)

func TestLoadKafkaConfiguration(t *testing.T) {
	setupConfig(t, "app_dev.yaml", `kafka:
  brokers:
    - 127.0.0.1:9092
  topic_mapping:
    social-forum: social-forum-dev
    hello: hello-dev
`)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Kafka.Brokers) != 1 || cfg.Kafka.Brokers[0] != "127.0.0.1:9092" ||
		cfg.Kafka.TopicMapping["social-forum"] != "social-forum-dev" ||
		cfg.Kafka.TopicMapping["hello"] != "hello-dev" {
		t.Fatalf("Kafka = %+v", cfg.Kafka)
	}
}

func TestLoadKafkaConsumerConfiguration(t *testing.T) {
	setupConfig(t, "app_dev.yaml", `kafka_consumer:
  brokers:
    - 127.0.0.1:9092
  group_name: goempty-hello
  topics:
    - hello
  topic_mapping:
    hello: hello-dev
  max_attempts: 5
  retry_backoff: 2s
`)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	consumerCfg := cfg.KafkaConsumer
	if len(consumerCfg.Brokers) != 1 || consumerCfg.Brokers[0] != "127.0.0.1:9092" ||
		consumerCfg.GroupName != "goempty-hello" ||
		!slices.Equal(consumerCfg.Topics, []string{"hello"}) ||
		consumerCfg.TopicMapping["hello"] != "hello-dev" ||
		consumerCfg.MaxAttempts != 5 ||
		consumerCfg.RetryBackoff != 2*time.Second {
		t.Fatalf("consumer config = %+v", consumerCfg)
	}
}
