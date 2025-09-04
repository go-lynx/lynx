package plugins

// HealthReport represents the detailed health status of a plugin
// Provides comprehensive health information for monitoring
type HealthReport struct {
	Status    string         // Current health status (healthy, degraded, unhealthy)
	Details   map[string]any // Detailed health metrics and information
	Timestamp int64          // Time of the health check (Unix timestamp)
	Message   string         // Optional descriptive message
}

// HealthCheck defines methods for plugin health monitoring
// Provides health status and monitoring capabilities
type HealthCheck interface {
	// GetHealth returns the current health status of the plugin
	// Provides detailed health information
	GetHealth() HealthReport
}
