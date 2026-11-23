package app

import (
	"goempty/internal/consumer"
	"goempty/internal/entity"
	"goempty/internal/service"
	"goempty/pkg/kafka"
)

type messagingDependencies struct {
	HelloService *service.HelloService
}

// newMessagingConsumers creates the Kafka consumer owned by App from the
// kafka_consumer configuration. An empty configuration means the app runs
// without a Kafka consumer.
func newMessagingConsumers(consumerCfg kafka.ConsumerConfig, deps *messagingDependencies) ([]*kafka.Consumer, error) {
	if consumerCfg.Brokers == nil && consumerCfg.GroupName == "" {
		return nil, nil
	}
	router := kafka.NewRouter()
	hello := consumer.NewHelloGreetingConsumer(deps.HelloService)
	if err := kafka.Subscribe(router, entity.HelloTopic, entity.HelloGreetingMQ, hello); err != nil {
		return nil, err
	}
	kafkaConsumer, err := kafka.NewConsumer(consumerCfg, router)
	if err != nil {
		return nil, err
	}
	return []*kafka.Consumer{kafkaConsumer}, nil
}
