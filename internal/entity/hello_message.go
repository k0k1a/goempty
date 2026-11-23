package entity

// HelloTopic is the logical Kafka topic hello messages use.
const HelloTopic = "hello"

// HelloGreetingMQ identifies hello greeting messages on HelloTopic.
const HelloGreetingMQ = "hello.greeting"

// HelloGreetingMessage is the data payload of HelloGreetingMQ messages and the
// body of the hello message publishing endpoint.
type HelloGreetingMessage struct {
	Message string `json:"message" binding:"required"`
}
