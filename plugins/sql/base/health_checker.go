package base

import (
	"context"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
)

// HealthCheckable interface for health checkable components
type HealthCheckable interface {
	CheckHealth() error
	Name() string
}

// Recoverable interface for components that can recover from failures
type Recoverable interface {
	Reconnect() error
	IsConnected() bool
}

// HealthChecker performs periodic health checks
type HealthChecker struct {
	target      HealthCheckable
	interval    time.Duration
	customQuery string

	mu            sync.Mutex
	lastCheck     time.Time
	isHealthy     bool
	failureCount  int64 // Count of consecutive failures
	maxFailures   int64 // Max failures before attempting recovery

	stopChan chan struct{}
	stopOnce sync.Once // Protect against multiple close operations
	stopped  bool
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(target HealthCheckable, interval time.Duration, customQuery string) *HealthChecker {
	return &HealthChecker{
		target:      target,
		interval:    interval,
		customQuery: customQuery,
		isHealthy:   true,
		maxFailures: 3, // Attempt recovery after 3 consecutive failures
		stopChan:    make(chan struct{}),
	}
}

// Start starts the health check routine
func (h *HealthChecker) Start(ctx context.Context) {
	go h.run(ctx)
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

// IsHealthy returns the current health status
func (h *HealthChecker) IsHealthy() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.isHealthy
}

// run performs periodic health checks
func (h *HealthChecker) run(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.performHealthCheck(ctx)
		case <-h.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// performHealthCheck performs a single health check
func (h *HealthChecker) performHealthCheck(ctx context.Context) {
	err := h.target.CheckHealth()

	h.mu.Lock()
	defer h.mu.Unlock()

	h.lastCheck = time.Now()

	if err != nil {
		h.failureCount++
		
		// Only log on state transition from healthy to unhealthy to avoid log spam
		if h.isHealthy {
			log.Errorf("Health check failed for %s: %v", h.target.Name(), err)
		}
		h.isHealthy = false

		// Attempt automatic recovery after consecutive failures
		if h.failureCount >= h.maxFailures {
			// Try to recover by reconnecting
			if recoverable, ok := h.target.(Recoverable); ok {
				log.Infof("Attempting automatic recovery for %s after %d consecutive failures", 
					h.target.Name(), h.failureCount)
				
				// Release lock before reconnecting to avoid deadlock
				h.mu.Unlock()
				reconnectErr := recoverable.Reconnect()
				h.mu.Lock()
				
				if reconnectErr == nil {
					log.Infof("Automatic recovery successful for %s", h.target.Name())
					h.failureCount = 0
					h.isHealthy = true
				} else {
					log.Warnf("Automatic recovery failed for %s: %v", h.target.Name(), reconnectErr)
				}
			}
		}
	} else {
		// Reset failure count on success
		h.failureCount = 0
		
		// Only log on state transition from unhealthy to healthy to avoid log spam
		if !h.isHealthy {
			log.Infof("Health check recovered for %s", h.target.Name())
		}
		h.isHealthy = true
	}
}
