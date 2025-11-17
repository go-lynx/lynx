// Package grpc provides connection pooling functionality for gRPC clients
package grpc

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// ConnectionSelectionStrategy defines how to select a connection from the pool
type ConnectionSelectionStrategy int

const (
	// RoundRobin selects connections in a round-robin fashion
	RoundRobin ConnectionSelectionStrategy = iota
	// Random selects a random connection
	Random
	// LeastUsed selects the connection with the least usage count
	LeastUsed
	// FirstAvailable selects the first available healthy connection
	FirstAvailable
)

// ConnectionPool manages a pool of gRPC connections for efficient reuse
// Supports multiple connections per service (channel pool) for better performance
type ConnectionPool struct {
	// Configuration
	maxServices        int                         // Maximum number of services in the pool
	maxConnsPerService int                         // Maximum connections per service
	idleTimeout        time.Duration               // Timeout for idle connections
	enabled            bool                        // Whether connection pooling is enabled
	selectionStrategy  ConnectionSelectionStrategy // Connection selection strategy

	// Pool management
	mu       sync.RWMutex
	services map[string]*serviceConnectionPool // Service name -> service connection pool
	metrics  *ClientMetrics                    // Metrics collector

	// Cleanup
	cleanupTicker  *time.Ticker
	stopCleanup    chan struct{}
	stopCleanupOnce sync.Once // Ensure stopCleanup is only closed once
}

// serviceConnectionPool manages multiple connections for a single service
type serviceConnectionPool struct {
	serviceName     string
	connections     []*pooledConnection
	mu              sync.RWMutex
	roundRobinIndex int64 // Atomic counter for round-robin selection
	createdAt       time.Time
	lastUsed        time.Time
}

// pooledConnection represents a connection with metadata
type pooledConnection struct {
	conn        *grpc.ClientConn
	serviceName string
	createdAt   time.Time
	lastUsed    time.Time
	useCount    int64 // Atomic counter for usage tracking
	mu          sync.RWMutex
}

// NewConnectionPool creates a new connection pool with the given configuration
func NewConnectionPool(maxServices int, maxConnsPerService int, idleTimeout time.Duration, enabled bool, metrics *ClientMetrics) *ConnectionPool {
	return NewConnectionPoolWithStrategy(maxServices, maxConnsPerService, idleTimeout, enabled, RoundRobin, metrics)
}

// NewConnectionPoolWithStrategy creates a new connection pool with selection strategy
func NewConnectionPoolWithStrategy(maxServices int, maxConnsPerService int, idleTimeout time.Duration, enabled bool, strategy ConnectionSelectionStrategy, metrics *ClientMetrics) *ConnectionPool {
	pool := &ConnectionPool{
		maxServices:        maxServices,
		maxConnsPerService: maxConnsPerService,
		idleTimeout:        idleTimeout,
		enabled:            enabled,
		selectionStrategy:  strategy,
		services:           make(map[string]*serviceConnectionPool),
		metrics:            metrics,
		stopCleanup:        make(chan struct{}),
	}

	if enabled && idleTimeout > 0 {
		// Start cleanup routine for idle connections
		pool.cleanupTicker = time.NewTicker(idleTimeout / 2)
		go pool.cleanupRoutine()
	}

	return pool
}

// GetConnection retrieves or creates a connection for the given service
// Returns one connection from the service's connection pool
func (p *ConnectionPool) GetConnection(serviceName string, createFunc func() (*grpc.ClientConn, error)) (*grpc.ClientConn, error) {
	return p.getConnectionWithRetry(serviceName, createFunc, 0)
}

// getConnectionWithRetry is the internal implementation with retry limit to prevent infinite recursion
const maxGetConnectionRetries = 3

func (p *ConnectionPool) getConnectionWithRetry(serviceName string, createFunc func() (*grpc.ClientConn, error), retryCount int) (*grpc.ClientConn, error) {
	if !p.enabled {
		// If pooling is disabled, create a new connection each time
		return createFunc()
	}

	// Prevent infinite recursion
	if retryCount >= maxGetConnectionRetries {
		log.Warnf("Service pool for %s was deleted multiple times, creating connection directly", serviceName)
		return createFunc()
	}

	p.mu.Lock()
	servicePool, exists := p.services[serviceName]
	if !exists {
		// Check if we've reached the maximum number of services
		if len(p.services) >= p.maxServices {
			// Evict least recently used service pool
			p.evictLRUService()
		}
		// Create new service pool
		servicePool = &serviceConnectionPool{
			serviceName: serviceName,
			connections: make([]*pooledConnection, 0),
			createdAt:   time.Now(),
			lastUsed:    time.Now(),
		}
		p.services[serviceName] = servicePool
	}
	p.mu.Unlock()

	// Re-check servicePool exists after releasing lock (defensive programming)
	// This handles the rare case where servicePool was deleted between lock release and use
	p.mu.RLock()
	servicePool, exists = p.services[serviceName]
	p.mu.RUnlock()

	if !exists {
		// Service pool was deleted (e.g., by cleanup or eviction), retry with limit
		// This should be very rare, but we handle it defensively
		log.Debugf("Service pool for %s was deleted, retrying connection creation (attempt %d/%d)", serviceName, retryCount+1, maxGetConnectionRetries)
		return p.getConnectionWithRetry(serviceName, createFunc, retryCount+1)
	}

	// Get connection from service pool
	return servicePool.GetConnection(p.selectionStrategy, p.maxConnsPerService, createFunc, p.metrics)
}

// GetConnection retrieves a connection from the service pool
func (sp *serviceConnectionPool) GetConnection(strategy ConnectionSelectionStrategy, maxConns int, createFunc func() (*grpc.ClientConn, error), metrics *ClientMetrics) (*grpc.ClientConn, error) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	now := time.Now()
	sp.lastUsed = now

	// Clean up unhealthy connections first
	sp.cleanupUnhealthy()

	// Select a healthy connection if available
	if len(sp.connections) > 0 {
		conn := sp.selectConnection(strategy)
		if conn != nil && isConnectionHealthy(conn.conn) {
			atomic.AddInt64(&conn.useCount, 1)
			conn.lastUsed = now
			if metrics != nil {
				metrics.RecordConnectionPoolHit(sp.serviceName)
			}
			return conn.conn, nil
		}
	}

	// Need to create a new connection
	if len(sp.connections) >= maxConns {
		// Remove least used connection to make room
		sp.evictLeastUsed()
	}

	// Create new connection
	conn, err := createFunc()
	if err != nil {
		if metrics != nil {
			metrics.RecordConnectionPoolMiss(sp.serviceName)
		}
		return nil, fmt.Errorf("failed to create connection for service %s: %w", sp.serviceName, err)
	}

	// Add to pool
	pooled := &pooledConnection{
		conn:        conn,
		serviceName: sp.serviceName,
		createdAt:   now,
		lastUsed:    now,
		useCount:    1,
	}
	sp.connections = append(sp.connections, pooled)

	if metrics != nil {
		metrics.RecordConnectionPoolMiss(sp.serviceName)
		metrics.RecordConnectionPoolSize(sp.serviceName, len(sp.connections))
	}

	return conn, nil
}

// selectConnection selects a connection based on the strategy
func (sp *serviceConnectionPool) selectConnection(strategy ConnectionSelectionStrategy) *pooledConnection {
	if len(sp.connections) == 0 {
		return nil
	}

	switch strategy {
	case RoundRobin:
		index := atomic.AddInt64(&sp.roundRobinIndex, 1) - 1
		return sp.connections[int(index)%len(sp.connections)]
	case Random:
		return sp.connections[rand.Intn(len(sp.connections))]
	case LeastUsed:
		var leastUsed *pooledConnection
		var minCount int64 = -1
		for _, conn := range sp.connections {
			count := atomic.LoadInt64(&conn.useCount)
			if minCount == -1 || count < minCount {
				minCount = count
				leastUsed = conn
			}
		}
		return leastUsed
	case FirstAvailable:
		for _, conn := range sp.connections {
			if isConnectionHealthy(conn.conn) {
				return conn
			}
		}
		return nil
	default:
		return sp.connections[0]
	}
}

// isConnectionHealthy checks if a connection is healthy
func isConnectionHealthy(conn *grpc.ClientConn) bool {
	state := conn.GetState()
	return state == connectivity.Ready || state == connectivity.Idle
}

// cleanupUnhealthy removes unhealthy connections from the pool
func (sp *serviceConnectionPool) cleanupUnhealthy() {
	healthy := make([]*pooledConnection, 0, len(sp.connections))
	for _, conn := range sp.connections {
		if isConnectionHealthy(conn.conn) {
			healthy = append(healthy, conn)
		} else {
			// Close unhealthy connection
			conn.mu.Lock()
			_ = conn.conn.Close()
			conn.mu.Unlock()
		}
	}
	sp.connections = healthy
}

// evictLeastUsed removes the least used connection
func (sp *serviceConnectionPool) evictLeastUsed() {
	if len(sp.connections) == 0 {
		return
	}

	var leastUsed *pooledConnection
	var minCount int64 = -1
	var minIndex int = -1

	for i, conn := range sp.connections {
		count := atomic.LoadInt64(&conn.useCount)
		if minCount == -1 || count < minCount {
			minCount = count
			leastUsed = conn
			minIndex = i
		}
	}

	if leastUsed != nil && minIndex >= 0 {
		leastUsed.mu.Lock()
		_ = leastUsed.conn.Close()
		leastUsed.mu.Unlock()
		// Remove from slice
		sp.connections = append(sp.connections[:minIndex], sp.connections[minIndex+1:]...)
	}
}

// CloseConnection closes all connections for a specific service
func (p *ConnectionPool) CloseConnection(serviceName string) error {
	if !p.enabled {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if servicePool, exists := p.services[serviceName]; exists {
		servicePool.mu.Lock()
		var lastErr error
		for _, conn := range servicePool.connections {
			conn.mu.Lock()
			if err := conn.conn.Close(); err != nil {
				lastErr = err
			}
			conn.mu.Unlock()
		}
		servicePool.mu.Unlock()
		delete(p.services, serviceName)
		if p.metrics != nil {
			p.metrics.RecordConnectionPoolSize(serviceName, 0)
		}
		return lastErr
	}

	return nil
}

// CloseAll closes all connections in the pool
func (p *ConnectionPool) CloseAll() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Stop cleanup routine (only once)
	if p.cleanupTicker != nil {
		p.cleanupTicker.Stop()
	}
	p.stopCleanupOnce.Do(func() {
		close(p.stopCleanup)
	})

	// Collect all service names before deletion for metrics recording
	serviceNames := make([]string, 0, len(p.services))
	for serviceName := range p.services {
		serviceNames = append(serviceNames, serviceName)
	}

	var lastErr error
	for serviceName, servicePool := range p.services {
		servicePool.mu.Lock()
		for _, conn := range servicePool.connections {
			conn.mu.Lock()
			if err := conn.conn.Close(); err != nil {
				lastErr = err
			}
			conn.mu.Unlock()
		}
		servicePool.mu.Unlock()
		delete(p.services, serviceName)
	}

	// Record metrics for all services that were closed
	if p.metrics != nil {
		for _, serviceName := range serviceNames {
			p.metrics.RecordConnectionPoolSize(serviceName, 0)
		}
	}

	return lastErr
}

// GetStats returns statistics about the connection pool
func (p *ConnectionPool) GetStats() map[string]interface{} {
	if !p.enabled {
		return map[string]interface{}{
			"enabled": false,
		}
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := map[string]interface{}{
		"enabled":               true,
		"max_services":          p.maxServices,
		"max_conns_per_service": p.maxConnsPerService,
		"current_services":      len(p.services),
		"idle_timeout":          p.idleTimeout.String(),
		"selection_strategy":    p.selectionStrategy.String(),
		"services":              make(map[string]interface{}),
	}

	services := make(map[string]interface{})
	for serviceName, servicePool := range p.services {
		servicePool.mu.RLock()
		serviceStats := map[string]interface{}{
			"connection_count": len(servicePool.connections),
			"created_at":       servicePool.createdAt,
			"last_used":        servicePool.lastUsed,
			"connections":      make([]map[string]interface{}, 0),
		}

		conns := make([]map[string]interface{}, 0, len(servicePool.connections))
		for _, conn := range servicePool.connections {
			conn.mu.RLock()
			conns = append(conns, map[string]interface{}{
				"created_at": conn.createdAt,
				"last_used":  conn.lastUsed,
				"use_count":  atomic.LoadInt64(&conn.useCount),
				"state":      conn.conn.GetState().String(),
			})
			conn.mu.RUnlock()
		}
		serviceStats["connections"] = conns
		servicePool.mu.RUnlock()
		services[serviceName] = serviceStats
	}
	stats["services"] = services

	return stats
}

// String returns string representation of ConnectionSelectionStrategy
func (s ConnectionSelectionStrategy) String() string {
	switch s {
	case RoundRobin:
		return "round_robin"
	case Random:
		return "random"
	case LeastUsed:
		return "least_used"
	case FirstAvailable:
		return "first_available"
	default:
		return "unknown"
	}
}

// evictLRUService removes the least recently used service pool
// Avoids deadlock by releasing locks before closing connections
func (p *ConnectionPool) evictLRUService() {
	p.mu.RLock()
	var oldestService string
	var oldestTime time.Time

	for serviceName, servicePool := range p.services {
		servicePool.mu.RLock()
		if oldestService == "" || servicePool.lastUsed.Before(oldestTime) {
			oldestService = serviceName
			oldestTime = servicePool.lastUsed
		}
		servicePool.mu.RUnlock()
	}
	p.mu.RUnlock()

	// Release lock before closing connections to avoid deadlock
	if oldestService != "" {
		// Use CloseConnection which handles locking properly
		_ = p.CloseConnection(oldestService)
	}
}

// cleanupRoutine periodically removes idle connections
func (p *ConnectionPool) cleanupRoutine() {
	for {
		select {
		case <-p.cleanupTicker.C:
			p.cleanupIdleConnections()
		case <-p.stopCleanup:
			return
		}
	}
}

// cleanupIdleConnections removes connections that have been idle for too long
// Avoids holding locks while closing connections to prevent blocking
func (p *ConnectionPool) cleanupIdleConnections() {
	p.mu.Lock()
	now := time.Now()
	var servicesToRemove []string
	var connectionsToClose []*pooledConnection
	var connectionsToKeep = make(map[string][]*pooledConnection)

	// Collect connections to close while holding locks
	for serviceName, servicePool := range p.services {
		servicePool.mu.Lock()
		// Check if service pool is idle
		if now.Sub(servicePool.lastUsed) > p.idleTimeout {
			servicesToRemove = append(servicesToRemove, serviceName)
			// Collect all connections for removal
			for _, conn := range servicePool.connections {
				connectionsToClose = append(connectionsToClose, conn)
			}
		} else {
			// Clean up idle connections within the service pool
			activeConns := make([]*pooledConnection, 0)
			for _, conn := range servicePool.connections {
				conn.mu.RLock()
				idle := now.Sub(conn.lastUsed) > p.idleTimeout
				conn.mu.RUnlock()

				if !idle {
					// Connection is still active, keep it
					activeConns = append(activeConns, conn)
				} else {
					// Mark for closing
					connectionsToClose = append(connectionsToClose, conn)
				}
			}
			connectionsToKeep[serviceName] = activeConns
		}
		servicePool.mu.Unlock()
	}
	p.mu.Unlock()

	// Close connections without holding locks
	for _, conn := range connectionsToClose {
		conn.mu.Lock()
		_ = conn.conn.Close()
		conn.mu.Unlock()
	}

	// Update service pools without holding main lock
	p.mu.Lock()
	for serviceName, activeConns := range connectionsToKeep {
		if servicePool, exists := p.services[serviceName]; exists {
			servicePool.mu.Lock()
			servicePool.connections = activeConns
			servicePool.mu.Unlock()
		}
	}

	// Remove idle service pools
	for _, serviceName := range servicesToRemove {
		delete(p.services, serviceName)
		if p.metrics != nil {
			p.metrics.RecordConnectionPoolSize(serviceName, 0)
		}
	}
	p.mu.Unlock()
}

// HealthCheck performs health checks on all pooled connections
func (p *ConnectionPool) HealthCheck(ctx context.Context) map[string]error {
	if !p.enabled {
		return make(map[string]error)
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	results := make(map[string]error)
	for serviceName, servicePool := range p.services {
		servicePool.mu.RLock()
		unhealthyCount := 0
		for _, conn := range servicePool.connections {
			conn.mu.RLock()
			state := conn.conn.GetState()
			if state == connectivity.Shutdown || state == connectivity.TransientFailure {
				unhealthyCount++
			}
			conn.mu.RUnlock()
		}
		if unhealthyCount > 0 {
			results[serviceName] = fmt.Errorf("%d unhealthy connections out of %d", unhealthyCount, len(servicePool.connections))
		}
		servicePool.mu.RUnlock()
	}

	return results
}
