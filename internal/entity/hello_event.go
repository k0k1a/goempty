package entity

const HelloGreetingRequested = "hello.greetingRequested"

// HelloGreetingEvent carries the greeting to log.
type HelloGreetingEvent struct {
	Message string
}

func (HelloGreetingEvent) Name() string { return HelloGreetingRequested }
