package rocketmq

import (
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
)

// ConnectionManager manages connection health and reconnection
type ConnectionManager struct {
	metrics       *Metrics
	healthChecker *HealthChecker
	mu            sync.RWMutex
	connected     bool
	stopCh        chan struct{}
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(metrics *Metrics) *ConnectionManager {
	return &ConnectionManager{
		metrics:       metrics,
		healthChecker: NewHealthChecker(metrics),
		stopCh:        make(chan struct{}),
	}
}

// Start starts the connection manager
func (cm *ConnectionManager) Start() {
	go cm.run()
}

// Stop stops the connection manager
func (cm *ConnectionManager) Stop() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	select {
	case <-cm.stopCh:
		// Already stopped
		return
	default:
		close(cm.stopCh)
	}

	cm.healthChecker.Stop()
}

// IsConnected checks if connected
func (cm *ConnectionManager) IsConnected() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.connected
}

// GetHealthChecker gets health checker
func (cm *ConnectionManager) GetHealthChecker() HealthCheckerInterface {
	return cm.healthChecker
}

// ForceReconnect forces reconnection
func (cm *ConnectionManager) ForceReconnect() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.connected = false
	cm.metrics.IncrementReconnectionCount()
	log.Info("Forced reconnection")
}

// run runs the connection manager loop
func (cm *ConnectionManager) run() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cm.stopCh:
			return
		case <-ticker.C:
			cm.checkConnection()
		}
	}
}

// checkConnection checks connection health
func (cm *ConnectionManager) checkConnection() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// For now, we'll assume connection is healthy if no errors
	// In a real implementation, you would check actual connection status
	cm.connected = true
}

// HealthChecker performs health checks
type HealthChecker struct {
	metrics     *Metrics
	mu          sync.RWMutex
	healthy     bool
	lastCheck   time.Time
	errorCount  int64
	stopCh      chan struct{}
	checkTicker *time.Ticker
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(metrics *Metrics) *HealthChecker {
	return &HealthChecker{
		metrics:     metrics,
		lastCheck:   time.Now(),
		stopCh:      make(chan struct{}),
		checkTicker: time.NewTicker(10 * time.Second),
	}
}

// Start starts health check
func (hc *HealthChecker) Start() {
	go hc.run()
}

// Stop stops health check
func (hc *HealthChecker) Stop() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	select {
	case <-hc.stopCh:
		// Already stopped
		return
	default:
		close(hc.stopCh)
	}

	hc.checkTicker.Stop()
}

// IsHealthy checks if healthy
func (hc *HealthChecker) IsHealthy() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.healthy
}

// GetLastCheck gets last check time
func (hc *HealthChecker) GetLastCheck() time.Time {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.lastCheck
}

// GetErrorCount gets error count
func (hc *HealthChecker) GetErrorCount() int {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return int(hc.errorCount)
}

// run runs the health check loop
func (hc *HealthChecker) run() {
	for {
		select {
		case <-hc.stopCh:
			return
		case <-hc.checkTicker.C:
			hc.performHealthCheck()
		}
	}
}

// performHealthCheck performs a health check
func (hc *HealthChecker) performHealthCheck() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.metrics.IncrementHealthCheckCount()
	hc.lastCheck = time.Now()
	hc.metrics.UpdateLastHealthCheck()

	// For now, we'll assume healthy if error count is low
	// In a real implementation, you would check actual service health
	if hc.errorCount < 5 {
		hc.healthy = true
		hc.metrics.SetHealthy(true)
	} else {
		hc.healthy = false
		hc.metrics.SetHealthy(false)
		hc.metrics.IncrementHealthCheckErrors()
		log.Warn("Health check failed", "errorCount", hc.errorCount)
	}
}
