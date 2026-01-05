package supervisor

// Logger defines the logging interface for supervisor output.
type Logger interface {
	Infof(format string, args ...any)
	Errorf(format string, args ...any)
}
