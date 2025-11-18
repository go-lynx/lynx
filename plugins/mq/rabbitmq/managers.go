package rabbitmq

import (
	"context"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins/mq/rabbitmq/conf"
)

// HealthChecker represents a health checker for RabbitMQ client
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
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		interval: 30 * time.Second,
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
	// Simple health check - in a real implementation, this would check RabbitMQ connectivity
	h.healthy = true
	h.lastError = nil
}

// ConnectionManager represents a connection manager for RabbitMQ client
type ConnectionManager struct {
	config    *conf.RabbitMQ
	connected bool
	stopChan  chan struct{}
	stopOnce  sync.Once // Protect against multiple close operations
	mu        sync.RWMutex
	stopped   bool
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(config *conf.RabbitMQ) *ConnectionManager {
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
	log.Infof("RabbitMQ connection manager started")
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
	log.Infof("RabbitMQ connection manager stopped")
}

// IsConnected returns the connection status
func (c *ConnectionManager) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// GetHealthChecker gets health checker
func (c *ConnectionManager) GetHealthChecker() HealthCheckerInterface {
	return nil // Return nil for now, could be implemented later
}

// ForceReconnect forces reconnection
func (c *ConnectionManager) ForceReconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	log.Infof("Forcing RabbitMQ reconnection")
	c.connected = false
	// In a real implementation, this would trigger a reconnection
	c.connected = true
}

// RetryHandler represents a retry handler for RabbitMQ operations
type RetryHandler struct {
	config *conf.RabbitMQ
}

// NewRetryHandler creates a new retry handler
func NewRetryHandler(config *conf.RabbitMQ) *RetryHandler {
	return &RetryHandler{
		config: config,
	}
}

// DoWithRetry executes operation with retry
func (r *RetryHandler) DoWithRetry(ctx context.Context, operation func() error) error {
	// Get retry configuration from the first producer (if available)
	maxRetries := int(defaultMaxRetries)
	backoffTime := 100 * time.Millisecond // defaultRetryBackoff

	if len(r.config.Producers) > 0 {
		maxRetries = int(r.config.Producers[0].MaxRetries)
		if r.config.Producers[0].RetryBackoff != nil {
			backoffTime = r.config.Producers[0].RetryBackoff.AsDuration()
		}
	}

	var lastErr error
	backoff := backoffTime

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute operation
		if err := operation(); err == nil {
			return nil
		} else {
			lastErr = err
		}

		// If this is the last attempt, don't wait
		if attempt == maxRetries {
			break
		}

		// Wait before retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		// Exponential backoff with max limit
		backoff *= 2
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
	}

	return WrapError(lastErr, "max retries exceeded")
}
