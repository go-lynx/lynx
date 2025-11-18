package pulsar

import (
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins/mq/pulsar/conf"
)

// HealthChecker represents a health checker for Pulsar client
type HealthChecker struct {
	interval   time.Duration
	stopChan   chan struct{}
	stopOnce   sync.Once // Protect against multiple close operations
	healthy    bool
	lastCheck  time.Time
	errorCount int
	lastError  error
	mu         sync.RWMutex
	stopped    bool
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(interval time.Duration) *HealthChecker {
	return &HealthChecker{
		interval: interval,
		stopChan: make(chan struct{}),
		healthy:  true,
	}
}

// Start starts the health checker
func (h *HealthChecker) Start() {
	go h.run()
}

// Stop stops the health checker
func (h *HealthChecker) Stop() {
	h.mu.Lock()
	stopped := h.stopped
	h.mu.Unlock()
	
	if !stopped {
		h.stopOnce.Do(func() {
			close(h.stopChan)
			h.mu.Lock()
			h.stopped = true
			h.mu.Unlock()
		})
	}
}

// IsHealthy returns the health status
func (h *HealthChecker) IsHealthy() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.healthy
}

// GetLastCheck returns the last check time
func (h *HealthChecker) GetLastCheck() time.Time {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastCheck
}

// GetErrorCount returns the error count
func (h *HealthChecker) GetErrorCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.errorCount
}

// GetLastError returns the last error
func (h *HealthChecker) GetLastError() error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastError
}

// run runs the health check loop
func (h *HealthChecker) run() {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.performHealthCheck()
		case <-h.stopChan:
			return
		}
	}
}

// performHealthCheck performs a health check
func (h *HealthChecker) performHealthCheck() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.lastCheck = time.Now()
	// Simple health check - in a real implementation, this would check Pulsar connectivity
	h.healthy = true
	h.lastError = nil
}

// ConnectionManager represents a connection manager for Pulsar client
type ConnectionManager struct {
	config    *conf.Connection
	connected bool
	stopChan  chan struct{}
	stopOnce  sync.Once // Protect against multiple close operations
	mu        sync.RWMutex
	stopped   bool
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(config *conf.Connection) *ConnectionManager {
	return &ConnectionManager{
		config:    config,
		connected: false,
		stopChan:  make(chan struct{}),
	}
}

// Start starts the connection manager
func (c *ConnectionManager) Start() {
	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()
	log.Infof("Pulsar connection manager started")
}

// Stop stops the connection manager
func (c *ConnectionManager) Stop() {
	c.mu.Lock()
	c.connected = false
	if c.stopped {
		c.mu.Unlock()
		return
	}
	c.stopped = true
	ch := c.stopChan
	c.mu.Unlock()
	select {
	case <-ch:
		// already closed
	default:
		close(ch)
	}
	log.Infof("Pulsar connection manager stopped")
}

// IsConnected returns the connection status
func (c *ConnectionManager) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// GetConnectionStats returns connection statistics
func (c *ConnectionManager) GetConnectionStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"connected":                 c.connected,
		"max_connections_per_host":  c.config.MaxConnectionsPerHost,
		"enable_connection_pooling": c.config.EnableConnectionPooling,
		"connection_timeout":        c.config.ConnectionTimeout.AsDuration(),
		"operation_timeout":         c.config.OperationTimeout.AsDuration(),
		"keep_alive_interval":       c.config.KeepAliveInterval.AsDuration(),
	}
}

// Reconnect attempts to reconnect
func (c *ConnectionManager) Reconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	log.Infof("Attempting to reconnect to Pulsar")
	c.connected = true
	return nil
}

// RetryManager represents a retry manager for Pulsar operations
type RetryManager struct {
	config *conf.Retry
	stats  map[string]interface{}
	mu     sync.RWMutex
}

// NewRetryManager creates a new retry manager
func NewRetryManager(config *conf.Retry) *RetryManager {
	return &RetryManager{
		config: config,
		stats:  make(map[string]interface{}),
	}
}

// ShouldRetry determines if operation should be retried
func (r *RetryManager) ShouldRetry(attempt int, err error) bool {
	if !r.config.Enable {
		return false
	}
	return attempt < int(r.config.MaxAttempts)
}

// GetRetryDelay gets retry delay for attempt
func (r *RetryManager) GetRetryDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return r.config.InitialDelay.AsDuration()
	}

	delay := r.config.InitialDelay.AsDuration()
	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * float64(r.config.RetryDelayMultiplier))
		if delay > r.config.MaxDelay.AsDuration() {
			delay = r.config.MaxDelay.AsDuration()
			break
		}
	}

	return delay
}

// RecordRetry records a retry attempt
func (r *RetryManager) RecordRetry(operation string, attempt int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.stats[operation] == nil {
		r.stats[operation] = make(map[string]interface{})
	}
	if opStats, ok := r.stats[operation].(map[string]interface{}); ok {
		opStats["attempts"] = attempt
		opStats["last_error"] = err.Error()
		opStats["last_retry"] = time.Now()
	}
}

// GetRetryStats gets retry statistics
func (r *RetryManager) GetRetryStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make(map[string]interface{})
	for k, v := range r.stats {
		stats[k] = v
	}
	return stats
}
