package plugins

// HealthReport represents the detailed health status of a plugin.
type HealthReport struct {
	Status    string         // Current health status (healthy, degraded, unhealthy)
	Details   map[string]any // Detailed health metrics and information
	Timestamp int64          // Time of the health check (Unix timestamp)
	Message   string         // Optional descriptive message
}

// HealthCheck defines methods for plugin health monitoring.
type HealthCheck interface {
	// GetHealth returns the current health status of the plugin.
	GetHealth() HealthReport
}
