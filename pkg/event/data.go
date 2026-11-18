package event

// Event is implemented by a business event. The publisher and handlers must
// keep the event name and data immutable until all handlers finish.
type Event interface {
	Name() string
}
