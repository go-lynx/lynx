package plugins

// HealthReport represents the detailed health status of a plugin
// Provides comprehensive health information for monitoring
// HealthReport 表示插件的详细健康状态。
// 提供全面的健康信息用于监控。
type HealthReport struct {
	Status    string         // Current health status (healthy, degraded, unhealthy) // 当前健康状态（健康、降级、不健康）
	Details   map[string]any // Detailed health metrics and information // 详细的健康指标和信息
	Timestamp int64          // Time of the health check (Unix timestamp) // 健康检查的时间（Unix 时间戳）
	Message   string         // Optional descriptive message // 可选的描述性消息
}

// HealthCheck defines methods for plugin health monitoring
// Provides health status and monitoring capabilities
// HealthCheck 定义了插件健康监控的方法。
// 提供健康状态和监控功能。
type HealthCheck interface {
	// GetHealth returns the current health status of the plugin
	// Provides detailed health information
	// GetHealth 返回插件的当前健康状态。
	// 提供详细的健康信息。
	GetHealth() HealthReport
}
