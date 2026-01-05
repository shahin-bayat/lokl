package process

type state int

const (
	stateStopped state = iota
	stateStarting
	stateRunning
	stateStopping
	stateFailed
)

func (s state) String() string {
	switch s {
	case stateStopped:
		return "stopped"
	case stateStarting:
		return "starting"
	case stateRunning:
		return "running"
	case stateStopping:
		return "stopping"
	case stateFailed:
		return "failed"
	default:
		return "unknown"
	}
}
