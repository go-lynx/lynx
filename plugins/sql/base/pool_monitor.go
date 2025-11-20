package base

import (
	"context"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
)

// PoolMonitor monitors connection pool health and triggers alerts
type PoolMonitor struct {
	target        Monitorable
	interval      time.Duration
	thresholds    *PoolThresholds
	mu            sync.Mutex
	lastAlert     time.Time
	alertCooldown time.Duration
	stopChan      chan struct{}
	stopOnce      sync.Once
	stopped       bool
	// Track alert severity to adjust cooldown
	lastSeverity string
}

// PoolThresholds defines alert thresholds for connection pool monitoring
type PoolThresholds struct {
	UsagePercentage float64 // Alert when pool usage exceeds this (0.0-1.0)
	WaitDuration    time.Duration // Alert when wait duration exceeds this
	WaitCount       int64   // Alert when wait count exceeds this
}

// Monitorable interface for components that can be monitored
type Monitorable interface {
	GetStats() *ConnectionPoolStats
	Name() string
}

// NewPoolMonitor creates a new connection pool monitor
func NewPoolMonitor(target Monitorable, interval time.Duration, thresholds *PoolThresholds) *PoolMonitor {
	if thresholds == nil {
		thresholds = &PoolThresholds{
			UsagePercentage: 0.8,              // 80%
			WaitDuration:    5 * time.Second,  // 5 seconds
			WaitCount:       10,                // 10 waits
		}
	}

	return &PoolMonitor{
		target:        target,
		interval:      interval,
		thresholds:    thresholds,
		alertCooldown: 60 * time.Second, // 1 minute cooldown between alerts
		stopChan:      make(chan struct{}),
	}
}

// Start starts the monitoring routine
func (m *PoolMonitor) Start(ctx context.Context) {
	go m.run(ctx)
}

// Stop stops the monitor
func (m *PoolMonitor) Stop() {
	m.mu.Lock()
	stopped := m.stopped
	m.mu.Unlock()

	if !stopped {
		m.stopOnce.Do(func() {
			close(m.stopChan)
			m.mu.Lock()
			m.stopped = true
			m.mu.Unlock()
		})
	}
}

// run performs periodic monitoring
func (m *PoolMonitor) run(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkAndAlert()
		case <-m.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// checkAndAlert checks pool stats and triggers alerts if thresholds are exceeded
func (m *PoolMonitor) checkAndAlert() {
	stats := m.target.GetStats()

	alerts := []string{}
	severity := "warning"

	// Check pool usage percentage
	if stats.MaxOpenConnections > 0 {
		usage := float64(stats.OpenConnections) / float64(stats.MaxOpenConnections)
		if usage >= m.thresholds.UsagePercentage {
			alerts = append(alerts, "high pool usage")
			if usage >= 0.95 {
				severity = "critical"
			}
		}
	}

	// Check wait duration
	if stats.WaitDuration > m.thresholds.WaitDuration {
		alerts = append(alerts, "high wait duration")
		if stats.WaitDuration > m.thresholds.WaitDuration*2 {
			severity = "critical"
		}
	}

	// Check wait count
	if stats.WaitCount > m.thresholds.WaitCount {
		alerts = append(alerts, "high wait count")
		if stats.WaitCount > m.thresholds.WaitCount*5 {
			severity = "critical"
		}
	}

	// Log alerts if any
	if len(alerts) > 0 {
		m.mu.Lock()
		// Adjust cooldown based on severity: critical alerts have shorter cooldown
		cooldown := m.alertCooldown
		if severity == "critical" && m.lastSeverity != "critical" {
			cooldown = 30 * time.Second // Shorter cooldown for critical alerts
		}
		shouldAlert := time.Since(m.lastAlert) > cooldown
		if shouldAlert {
			m.lastAlert = time.Now()
			m.lastSeverity = severity
		}
		m.mu.Unlock()

		if shouldAlert {
			log.Warnf("Connection pool alert [%s] for %s: %v (Open=%d/%d, InUse=%d, Idle=%d, WaitCount=%d, WaitDuration=%v)",
				severity,
				m.target.Name(),
				alerts,
				stats.OpenConnections,
				stats.MaxOpenConnections,
				stats.InUse,
				stats.Idle,
				stats.WaitCount,
				stats.WaitDuration,
			)
		}
	} else {
		// Reset severity if no alerts
		m.mu.Lock()
		if m.lastSeverity != "" {
			m.lastSeverity = ""
		}
		m.mu.Unlock()
	}
}

