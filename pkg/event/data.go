package event

// Event describes a business occurrence, for example "user.registered".
// Payload is owned by the publisher and must remain immutable until all
// handlers finish. Business payload types belong in internal/entity; this
// package only defines the generic envelope and dispatch infrastructure.
type Event struct {
	Name    string
	Payload any
}
