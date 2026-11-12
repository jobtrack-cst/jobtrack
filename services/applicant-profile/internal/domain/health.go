package domain

// HealthStatus describes whether the service can handle requests.
type HealthStatus uint8

const (
	Unknown HealthStatus = iota
	Serving
	NotServing
)

func (s HealthStatus) IsHealthy() bool { return s == Serving }
