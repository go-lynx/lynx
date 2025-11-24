package base

import (
	"context"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
)

// LeakDetector detects connection leaks by monitoring connection usage
type LeakDetector struct {
	target    Monitorable
	threshold time.Duration
	mu        sync.Mutex
	stopChan  chan struct{}
	stopOnce  sync.Once
	stopped   bool
}

// NewLeakDetector creates a new connection leak detector
func NewLeakDetector(target Monitorable, threshold time.Duration) *LeakDetector {
	return &LeakDetector{
		target:    target,
		threshold: threshold,
		stopChan:  make(chan struct{}),
	}
}

// Start starts the leak detection routine
func (l *LeakDetector) Start(ctx context.Context) {
	go l.run(ctx)
}

// Stop stops the leak detector
func (l *LeakDetector) Stop() {
	l.mu.Lock()
	stopped := l.stopped
	l.mu.Unlock()

	if !stopped {
		l.stopOnce.Do(func() {
			close(l.stopChan)
			l.mu.Lock()
			l.stopped = true
			l.mu.Unlock()
		})
	}
}

// run performs periodic leak detection
func (l *LeakDetector) run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.detectLeaks()
		case <-l.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// detectLeaks checks for potential connection leaks
func (l *LeakDetector) detectLeaks() {
	stats := l.target.GetStats()

	// Check if connections are in use for too long
	// This is a simplified check - in a real implementation, we'd track individual connections
	if stats.InUse > 0 {
		// If connections are in use and pool is near capacity, it might indicate leaks
		if stats.MaxOpenConnections > 0 {
			usage := float64(stats.OpenConnections) / float64(stats.MaxOpenConnections)
			if usage >= 0.9 && stats.InUse == stats.OpenConnections {
				// All connections are in use, potential leak
				log.Warnf("Potential connection leak detected for %s: all connections (%d/%d) are in use",
					l.target.Name(), stats.OpenConnections, stats.MaxOpenConnections)
			}
		}

		// Check wait duration - long waits might indicate leaks
		if stats.WaitDuration > l.threshold {
			log.Warnf("Long connection wait detected for %s: %v (threshold: %v). Possible connection leak.",
				l.target.Name(), stats.WaitDuration, l.threshold)
		}
	}
}

