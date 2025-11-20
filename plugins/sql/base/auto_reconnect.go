package base

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-lynx/lynx/app/log"
)

// Reconnectable interface for components that can be reconnected
type Reconnectable interface {
	Reconnect() error
	IsConnected() bool
	Name() string
}

// AutoReconnector performs automatic reconnection on connection loss
type AutoReconnector struct {
	target      Reconnectable
	interval    time.Duration
	maxAttempts int
	mu          sync.Mutex
	attempts    int64
	reconnecting atomic.Bool
	stopChan    chan struct{}
	stopOnce    sync.Once
	stopped     bool
}

// NewAutoReconnector creates a new auto-reconnector
func NewAutoReconnector(target Reconnectable, interval time.Duration, maxAttempts int) *AutoReconnector {
	return &AutoReconnector{
		target:      target,
		interval:    interval,
		maxAttempts: maxAttempts,
		stopChan:    make(chan struct{}),
	}
}

// Start starts the auto-reconnect routine
func (a *AutoReconnector) Start(ctx context.Context) {
	go a.run(ctx)
}

// Stop stops the auto-reconnector
func (a *AutoReconnector) Stop() {
	a.mu.Lock()
	stopped := a.stopped
	a.mu.Unlock()

	if !stopped {
		a.stopOnce.Do(func() {
			close(a.stopChan)
			a.mu.Lock()
			a.stopped = true
			a.mu.Unlock()
		})
	}
}

// run performs periodic connection checks and reconnection
func (a *AutoReconnector) run(ctx context.Context) {
	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.checkAndReconnect()
		case <-a.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// checkAndReconnect checks connection status and attempts reconnection if needed
func (a *AutoReconnector) checkAndReconnect() {
	// Skip if already reconnecting
	if a.reconnecting.Load() {
		return
	}

	// Check if connection is actually down
	if a.target.IsConnected() {
		// Connection is healthy, reset attempts
		a.mu.Lock()
		a.attempts = 0
		a.mu.Unlock()
		return
	}

	// Connection is down, check if we should attempt reconnection
	a.mu.Lock()
	currentAttempts := a.attempts
	a.mu.Unlock()

	// Check max attempts limit (0 means unlimited)
	if a.maxAttempts > 0 && currentAttempts >= int64(a.maxAttempts) {
		return // Max attempts reached
	}

	// Attempt reconnection
	a.reconnecting.Store(true)
	defer a.reconnecting.Store(false)

	log.Infof("Attempting to reconnect %s (attempt %d)", a.target.Name(), currentAttempts+1)

	if err := a.target.Reconnect(); err != nil {
		a.mu.Lock()
		a.attempts++
		attempts := a.attempts
		a.mu.Unlock()

		log.Warnf("Reconnection attempt %d failed for %s: %v", attempts, a.target.Name(), err)
	} else {
		a.mu.Lock()
		a.attempts = 0
		a.mu.Unlock()

		log.Infof("Successfully reconnected %s", a.target.Name())
	}
}

// GetAttempts returns the current number of reconnection attempts
func (a *AutoReconnector) GetAttempts() int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.attempts
}

