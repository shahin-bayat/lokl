package types

// EventType represents the type of service event.
type EventType int

const (
	EventServiceStateChanged EventType = iota
	EventServiceHealthChanged
)

// Event represents a service state change notification.
type Event struct {
	Type    EventType
	Service string
}
