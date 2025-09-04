package redis

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Prometheus metrics
var (
	redisStartupTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "lynx",
		Subsystem: "redis_client",
		Name:      "startup_total",
		Help:      "Total number of Redis client startups attempted.",
	})
	redisStartupFailedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "lynx",
		Subsystem: "redis_client",
		Name:      "startup_failed_total",
		Help:      "Total number of Redis client startup failures.",
	})
	redisPingLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "lynx",
		Subsystem: "redis_client",
		Name:      "ping_latency_seconds",
		Help:      "Latency of Redis PING operations.",
		Buckets:   prometheus.DefBuckets,
	})
	// Pool stats gauges
	redisPoolHits = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "lynx",
		Subsystem: "redis_client",
		Name:      "pool_hits_total",
		Help:      "Total number of times a free connection was found in the pool.",
	})
	redisPoolMisses = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "lynx",
		Subsystem: "redis_client",
		Name:      "pool_misses_total",
		Help:      "Total number of times a free connection was NOT found in the pool.",
	})
	redisPoolTimeouts = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "lynx",
		Subsystem: "redis_client",
		Name:      "pool_timeouts_total",
		Help:      "Total number of connection timeouts.",
	})
	redisPoolTotalConns = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "lynx",
		Subsystem: "redis_client",
		Name:      "pool_total_conns",
		Help:      "The total number of connections in the pool.",
	})
	redisPoolIdleConns = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "lynx",
		Subsystem: "redis_client",
		Name:      "pool_idle_conns",
		Help:      "The number of idle connections.",
	})
	redisPoolStaleConns = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "lynx",
		Subsystem: "redis_client",
		Name:      "pool_stale_conns",
		Help:      "The number of stale connections removed from the pool.",
	})
	// Command level metrics
	redisCmdLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "lynx",
		Subsystem: "redis_client",
		Name:      "cmd_latency_seconds",
		Help:      "Latency of Redis commands by name.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"cmd"})
	redisCmdErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "lynx",
		Subsystem: "redis_client",
		Name:      "cmd_errors_total",
		Help:      "Total number of Redis command errors by name.",
	}, []string{"cmd"})

	// Health/info metrics
	redisClusterState = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "lynx",
		Subsystem: "redis_client",
		Name:      "cluster_state",
		Help:      "Cluster state: 1 ok, 0 fail (only in cluster mode).",
	})
	redisIsMaster = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "lynx",
		Subsystem: "redis_client",
		Name:      "is_master",
		Help:      "Role: 1 master, 0 replica (from INFO replication).",
	})
	redisConnectedSlaves = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "lynx",
		Subsystem: "redis_client",
		Name:      "connected_slaves",
		Help:      "Number of connected replicas (from INFO replication).",
	})
	redisServerInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "lynx",
		Subsystem: "redis_client",
		Name:      "server_info",
		Help:      "Static server info labeled with version, mode, client_name.",
	}, []string{"version", "mode", "client_name"})
)

func init() {
	// Best effort register; ignore duplicate registration panics by using MustRegister once per process
	prometheus.MustRegister(
		redisStartupTotal,
		redisStartupFailedTotal,
		redisPingLatency,
		redisPoolHits,
		redisPoolMisses,
		redisPoolTimeouts,
		redisPoolTotalConns,
		redisPoolIdleConns,
		redisPoolStaleConns,
		redisCmdLatency,
		redisCmdErrors,
		redisClusterState,
		redisIsMaster,
		redisConnectedSlaves,
		redisServerInfo,
	)
}
