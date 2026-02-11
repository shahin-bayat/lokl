package types

// EventType represents the type of service event.
type EventType int

const (
	EventServiceStateChanged EventType = iota
	EventServiceHealthChanged
	EventProgress
	EventError
)

type Event struct {
	Type    EventType
	Service string
	Message string
}
