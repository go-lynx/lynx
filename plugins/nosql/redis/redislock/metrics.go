package redislock

import (
	"errors"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds Prometheus collectors for the redis_lock package.
// Call InitMetrics once at program start to register them (or use the default Registerer).
var (
	lockAcquireTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "lynx",
			Subsystem: "redis_lock",
			Name:      "acquire_total",
			Help:      "Total number of lock acquire attempts partitioned by result.",
		},
		[]string{"result"}, // success|conflict|error
	)

	lockUnlockTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "lynx",
			Subsystem: "redis_lock",
			Name:      "unlock_total",
			Help:      "Total number of unlock attempts partitioned by result.",
		},
		[]string{"result"}, // full|partial|not_held|error
	)

	lockRenewTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "lynx",
			Subsystem: "redis_lock",
			Name:      "renew_total",
			Help:      "Total number of renew attempts partitioned by result.",
		},
		[]string{"result"}, // success|not_owner|not_exist|fail|error
	)

	skippedRenewalsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "lynx",
			Subsystem: "redis_lock",
			Name:      "skipped_renewals_total",
			Help:      "Total number of renew tasks skipped due to worker pool saturation.",
		},
	)

	activeLocks = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "lynx",
			Subsystem: "redis_lock",
			Name:      "active_locks",
			Help:      "Current number of active locks managed for renewal.",
		},
	)

	scriptLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "lynx",
			Subsystem: "redis_lock",
			Name:      "script_latency_seconds",
			Help:      "Latency of Redis Lua script calls.",
			Buckets:   prometheus.DefBuckets, // [0.005 .. 10] seconds
		},
		[]string{"op"}, // acquire|unlock|renew
	)
)

// InitMetrics registers the collectors to the provided Registerer.
// Pass nil to use the default Registerer.
func InitMetrics(reg prometheus.Registerer) {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	// Use Register and ignore AlreadyRegistered errors to avoid panic during duplicate initialization
	if err := reg.Register(lockAcquireTotal); err != nil {
		var alreadyRegisteredError prometheus.AlreadyRegisteredError
		if !errors.As(err, &alreadyRegisteredError) {
			log.Errorf("Failed to register lockAcquireTotal metric: %v", err)
			return
		}
	}
	if err := reg.Register(lockUnlockTotal); err != nil {
		var alreadyRegisteredError prometheus.AlreadyRegisteredError
		if !errors.As(err, &alreadyRegisteredError) {
			log.Errorf("Failed to register lockUnlockTotal metric: %v", err)
			return
		}
	}
	if err := reg.Register(lockRenewTotal); err != nil {
		var alreadyRegisteredError prometheus.AlreadyRegisteredError
		if !errors.As(err, &alreadyRegisteredError) {
			log.Errorf("Failed to register lockRenewTotal metric: %v", err)
			return
		}
	}
	if err := reg.Register(skippedRenewalsTotal); err != nil {
		var alreadyRegisteredError prometheus.AlreadyRegisteredError
		if !errors.As(err, &alreadyRegisteredError) {
			log.Errorf("Failed to register skippedRenewalsTotal metric: %v", err)
			return
		}
	}
	if err := reg.Register(activeLocks); err != nil {
		var alreadyRegisteredError prometheus.AlreadyRegisteredError
		if !errors.As(err, &alreadyRegisteredError) {
			log.Errorf("Failed to register activeLocks metric: %v", err)
			return
		}
	}
	if err := reg.Register(scriptLatency); err != nil {
		var alreadyRegisteredError prometheus.AlreadyRegisteredError
		if !errors.As(err, &alreadyRegisteredError) {
			log.Errorf("Failed to register scriptLatency metric: %v", err)
			return
		}
	}
}

// Helper functions used within the package to record metrics.
func observeScriptLatency(op string, d time.Duration) {
	scriptLatency.WithLabelValues(op).Observe(d.Seconds())
}

func incAcquire(result string) { lockAcquireTotal.WithLabelValues(result).Inc() }
func incUnlock(result string)  { lockUnlockTotal.WithLabelValues(result).Inc() }
func incRenew(result string)   { lockRenewTotal.WithLabelValues(result).Inc() }

func incSkippedRenewal() { skippedRenewalsTotal.Inc() }

func activeLocksInc() { activeLocks.Inc() }
func activeLocksDec() { activeLocks.Dec() }
