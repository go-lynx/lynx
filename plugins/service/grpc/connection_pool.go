// Package grpc provides connection pooling functionality for gRPC clients
package grpc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"google.golang.org/grpc"
)

// ConnectionPool manages a pool of gRPC connections for efficient reuse
type ConnectionPool struct {
	// Configuration
	maxSize     int           // Maximum number of connections in the pool
	idleTimeout time.Duration // Timeout for idle connections
	enabled     bool          // Whether connection pooling is enabled

	// Pool management
	mu          sync.RWMutex
	connections map[string]*pooledConnection // Service name -> pooled connection
	metrics     *ClientMetrics               // Metrics collector

	// Cleanup
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// pooledConnection represents a connection with metadata
type pooledConnection struct {
	conn        *grpc.ClientConn
	serviceName string
	createdAt   time.Time
	lastUsed    time.Time
	useCount    int64
	mu          sync.RWMutex
}

// NewConnectionPool creates a new connection pool with the given configuration
func NewConnectionPool(maxSize int, idleTimeout time.Duration, enabled bool, metrics *ClientMetrics) *ConnectionPool {
	pool := &ConnectionPool{
		maxSize:     maxSize,
		idleTimeout: idleTimeout,
		enabled:     enabled,
		connections: make(map[string]*pooledConnection),
		metrics:     metrics,
		stopCleanup: make(chan struct{}),
	}

	if enabled && idleTimeout > 0 {
		// Start cleanup routine for idle connections
		pool.cleanupTicker = time.NewTicker(idleTimeout / 2)
		go pool.cleanupRoutine()
	}

	return pool
}

// GetConnection retrieves or creates a connection for the given service
func (p *ConnectionPool) GetConnection(serviceName string, createFunc func() (*grpc.ClientConn, error)) (*grpc.ClientConn, error) {
	if !p.enabled {
		// If pooling is disabled, create a new connection each time
		return createFunc()
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if we have an existing connection
	if pooled, exists := p.connections[serviceName]; exists {
		pooled.mu.Lock()
		// Check if connection is still valid
		if pooled.conn.GetState().String() != "SHUTDOWN" {
			pooled.lastUsed = time.Now()
			pooled.useCount++
			pooled.mu.Unlock()

			// Update metrics
			if p.metrics != nil {
				p.metrics.RecordConnectionPoolHit(serviceName)
			}

			return pooled.conn, nil
		}
		// Connection is invalid, remove it
		pooled.mu.Unlock()
		delete(p.connections, serviceName)
		if p.metrics != nil {
			p.metrics.RecordConnectionPoolSize(len(p.connections))
		}
	}

	// Check pool size limit
	if len(p.connections) >= p.maxSize {
		// Find and remove the least recently used connection
		p.evictLRU()
	}

	// Create new connection
	conn, err := createFunc()
	if err != nil {
		if p.metrics != nil {
			p.metrics.RecordConnectionPoolMiss(serviceName)
		}
		return nil, fmt.Errorf("failed to create connection for service %s: %w", serviceName, err)
	}

	// Add to pool
	now := time.Now()
	pooled := &pooledConnection{
		conn:        conn,
		serviceName: serviceName,
		createdAt:   now,
		lastUsed:    now,
		useCount:    1,
	}

	p.connections[serviceName] = pooled

	// Update metrics
	if p.metrics != nil {
		p.metrics.RecordConnectionPoolMiss(serviceName)
		p.metrics.RecordConnectionPoolSize(len(p.connections))
	}

	return conn, nil
}

// CloseConnection closes a specific connection and removes it from the pool
func (p *ConnectionPool) CloseConnection(serviceName string) error {
	if !p.enabled {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if pooled, exists := p.connections[serviceName]; exists {
		pooled.mu.Lock()
		err := pooled.conn.Close()
		pooled.mu.Unlock()

		delete(p.connections, serviceName)
		if p.metrics != nil {
			p.metrics.RecordConnectionPoolSize(len(p.connections))
		}

		return err
	}

	return nil
}

// CloseAll closes all connections in the pool
func (p *ConnectionPool) CloseAll() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Stop cleanup routine
	if p.cleanupTicker != nil {
		p.cleanupTicker.Stop()
		close(p.stopCleanup)
	}

	var lastErr error
	for serviceName, pooled := range p.connections {
		pooled.mu.Lock()
		if err := pooled.conn.Close(); err != nil {
			lastErr = err
		}
		pooled.mu.Unlock()
		delete(p.connections, serviceName)
	}

	if p.metrics != nil {
		p.metrics.RecordConnectionPoolSize(0)
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
		"enabled":      true,
		"max_size":     p.maxSize,
		"current_size": len(p.connections),
		"idle_timeout": p.idleTimeout.String(),
		"connections":  make(map[string]interface{}),
	}

	connections := make(map[string]interface{})
	for serviceName, pooled := range p.connections {
		pooled.mu.RLock()
		connections[serviceName] = map[string]interface{}{
			"created_at": pooled.createdAt,
			"last_used":  pooled.lastUsed,
			"use_count":  pooled.useCount,
			"state":      pooled.conn.GetState().String(),
		}
		pooled.mu.RUnlock()
	}
	stats["connections"] = connections

	return stats
}

// evictLRU removes the least recently used connection from the pool
func (p *ConnectionPool) evictLRU() {
	var oldestService string
	var oldestTime time.Time

	for serviceName, pooled := range p.connections {
		pooled.mu.RLock()
		if oldestService == "" || pooled.lastUsed.Before(oldestTime) {
			oldestService = serviceName
			oldestTime = pooled.lastUsed
		}
		pooled.mu.RUnlock()
	}

	if oldestService != "" {
		if pooled, exists := p.connections[oldestService]; exists {
			pooled.mu.Lock()
			err := pooled.conn.Close()
			if err != nil {
				log.Error(err)
				return
			}
			pooled.mu.Unlock()
			delete(p.connections, oldestService)
		}
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
func (p *ConnectionPool) cleanupIdleConnections() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	var toRemove []string

	for serviceName, pooled := range p.connections {
		pooled.mu.RLock()
		if now.Sub(pooled.lastUsed) > p.idleTimeout {
			toRemove = append(toRemove, serviceName)
		}
		pooled.mu.RUnlock()
	}

	for _, serviceName := range toRemove {
		if pooled, exists := p.connections[serviceName]; exists {
			pooled.mu.Lock()
			err := pooled.conn.Close()
			if err != nil {
				log.Error(err)
				return
			}
			pooled.mu.Unlock()
			delete(p.connections, serviceName)
		}
	}

	if len(toRemove) > 0 && p.metrics != nil {
		p.metrics.RecordConnectionPoolSize(len(p.connections))
	}
}

// HealthCheck performs health checks on all pooled connections
func (p *ConnectionPool) HealthCheck(ctx context.Context) map[string]error {
	if !p.enabled {
		return make(map[string]error)
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	results := make(map[string]error)
	for serviceName, pooled := range p.connections {
		pooled.mu.RLock()
		state := pooled.conn.GetState()
		if state.String() == "SHUTDOWN" || state.String() == "TRANSIENT_FAILURE" {
			results[serviceName] = fmt.Errorf("connection in unhealthy state: %s", state.String())
		}
		pooled.mu.RUnlock()
	}

	return results
}
