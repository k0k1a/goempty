package config

import "testing"

func TestLoadKafkaConfiguration(t *testing.T) {
	setupConfig(t, "app_dev.yaml", `kafka:
  brokers:
    - 127.0.0.1:9092
  topic_mapping:
    social-forum: social-forum-dev
    forum.events: Forum.Events.Dev
`)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Kafka.Brokers) != 1 || cfg.Kafka.Brokers[0] != "127.0.0.1:9092" ||
		cfg.Kafka.TopicMapping["social-forum"] != "social-forum-dev" ||
		cfg.Kafka.TopicMapping["forum.events"] != "Forum.Events.Dev" {
		t.Fatalf("Kafka = %+v", cfg.Kafka)
	}
}
