package app

import "goempty/pkg/kafka"

// newMessagingConsumers creates the Kafka consumers owned by App. The template
// registers none; add one kafka.NewConsumer call per consumer entry and extend
// this function with the services and configuration they need.
func newMessagingConsumers() ([]*kafka.Consumer, error) {
	return nil, nil
}
