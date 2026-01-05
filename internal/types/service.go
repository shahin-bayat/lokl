package types

// ServiceInfo represents a service's current state.
type ServiceInfo struct {
	Name    string
	Domain  string
	Port    int
	Running bool
	Healthy bool
}
