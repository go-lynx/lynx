package redis

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/redis/go-redis/v9"
)

// detectMode returns single/cluster/sentinel
func (r *PlugRedis) detectMode() string {
	if r.conf.Sentinel != nil && r.conf.Sentinel.MasterName != "" {
		return "sentinel"
	}
	addrList := r.currentAddrList()
	if len(addrList) > 1 {
		return "cluster"
	}
	return "single"
}

func (r *PlugRedis) currentAddrList() []string {
	if r.conf.Sentinel != nil && len(r.conf.Sentinel.Addrs) > 0 {
		return append([]string{}, r.conf.Sentinel.Addrs...)
	}
	if len(r.conf.Addrs) > 0 {
		return append([]string{}, r.conf.Addrs...)
	}
	return nil
}

// enhancedReadinessCheck performs stricter readiness checks based on mode
func (r *PlugRedis) enhancedReadinessCheck(mode string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	switch c := r.rdb.(type) {
	case *redis.ClusterClient:
		info, err := c.Info(ctx, "cluster").Result()
		if err == nil && strings.Contains(info, "cluster_state:ok") {
			redisClusterState.Set(1)
			log.Infof("redis cluster readiness ok: cluster_state=ok")
		} else {
			redisClusterState.Set(0)
			if err != nil {
				log.Warnf("redis cluster readiness check failed: %v", err)
			} else {
				log.Warnf("redis cluster readiness: state not ok")
			}
		}
	default:
		// Single node/Sentinel: read INFO replication to determine role
		info, err := r.rdb.Info(ctx, "replication").Result()
		if err == nil {
			if strings.Contains(info, "role:master") {
				redisIsMaster.Set(1)
			} else {
				redisIsMaster.Set(0)
			}
			// Parse connected_slaves:N
			if idx := strings.Index(info, "connected_slaves:"); idx >= 0 {
				rest := info[idx+len("connected_slaves:"):]
				n := 0
				for i := 0; i < len(rest); i++ {
					if rest[i] < '0' || rest[i] > '9' {
						break
					}
					n = n*10 + int(rest[i]-'0')
				}
				redisConnectedSlaves.Set(float64(n))
			}
		}
	}
	// Write server_info metrics once
	version := r.readRedisVersion()
	redisServerInfo.WithLabelValues(version, mode, r.conf.ClientName).Set(1)
}

func (r *PlugRedis) readRedisVersion() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	info, err := r.rdb.Info(ctx, "server").Result()
	if err != nil {
		return "unknown"
	}
	// find redis_version:6.2.0
	if idx := strings.Index(info, "redis_version:"); idx >= 0 {
		rest := info[idx+len("redis_version:"):]
		// read until newline
		for i := 0; i < len(rest); i++ {
			if rest[i] == '\n' || rest[i] == '\r' {
				return strings.TrimSpace(rest[:i])
			}
		}
		return strings.TrimSpace(rest)
	}
	return "unknown"
}

// startInfoCollector periodically collects INFO and cluster status
func (r *PlugRedis) startInfoCollector(mode string) {
	r.statsWG.Add(1)
	quit := r.statsQuit
	go func() {
		defer r.statsWG.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-quit:
				return
			case <-ticker.C:
				r.enhancedReadinessCheck(mode)
			}
		}
	}()
}

// HealthStatus returns detailed health status information
type HealthStatus struct {
	Healthy      bool          `json:"healthy"`
	Mode         string        `json:"mode"`
	Latency      time.Duration `json:"latency"`
	Version      string        `json:"version"`
	ConnectedNow int           `json:"connected_now"`
	IsMaster     bool          `json:"is_master"`
	ClusterOK    bool          `json:"cluster_ok,omitempty"`
	Error        string        `json:"error,omitempty"`
}

// GetHealthStatus get detailed health status
func (r *PlugRedis) GetHealthStatus(ctx context.Context) *HealthStatus {
	if r.rdb == nil {
		return &HealthStatus{
			Healthy: false,
			Error:   "redis client not initialized",
		}
	}

	status := &HealthStatus{
		Mode: r.detectMode(),
	}

	// Ping test
	start := time.Now()
	if _, err := r.rdb.Ping(ctx).Result(); err != nil {
		status.Healthy = false
		status.Error = err.Error()
		return status
	}
	status.Latency = time.Since(start)
	status.Healthy = true

	// Get version information
	status.Version = r.readRedisVersion()

	// Get connection information
	// Note: GetPoolStats method may need to be implemented or use alternative approach
	// if poolStats := r.GetPoolStats(); poolStats != nil {
	//     status.ConnectedNow = int(poolStats.TotalConns)
	// }

	// Get master-slave status
	info, _ := r.rdb.Info(ctx, "replication").Result()
	status.IsMaster = strings.Contains(info, "role:master")

	// Cluster status
	if clusterClient, ok := r.rdb.(*redis.ClusterClient); ok {
		clusterInfo, _ := clusterClient.Info(ctx, "cluster").Result()
		status.ClusterOK = strings.Contains(clusterInfo, "cluster_state:ok")
	}

	return status
}

// PerformanceMetrics performance metrics
type PerformanceMetrics struct {
	CommandsProcessed int64         `json:"commands_processed"`
	UsedMemory        int64         `json:"used_memory"`
	ConnectedClients  int           `json:"connected_clients"`
	HitRate           float64       `json:"hit_rate"`
	EvictedKeys       int64         `json:"evicted_keys"`
	ExpiredKeys       int64         `json:"expired_keys"`
	AvgLatency        time.Duration `json:"avg_latency"`
}

// GetPerformanceMetrics get performance metrics
func (r *PlugRedis) GetPerformanceMetrics(ctx context.Context) (*PerformanceMetrics, error) {
	if r.rdb == nil {
		return nil, fmt.Errorf("redis client not initialized")
	}

	metrics := &PerformanceMetrics{}

	// Get statistics information
	info, err := r.rdb.Info(ctx, "stats").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	// Parse statistics information
	parseIntFromInfo := func(info, key string) int64 {
		if idx := strings.Index(info, key+":"); idx >= 0 {
			rest := info[idx+len(key)+1:]
			var val int64
			_, err := fmt.Sscanf(rest, "%d", &val)
			if err != nil {
				log.Error(err)
				return 0
			}
			return val
		}
		return 0
	}

	metrics.CommandsProcessed = parseIntFromInfo(info, "total_commands_processed")
	metrics.ExpiredKeys = parseIntFromInfo(info, "expired_keys")
	metrics.EvictedKeys = parseIntFromInfo(info, "evicted_keys")

	// Get memory information
	memInfo, _ := r.rdb.Info(ctx, "memory").Result()
	metrics.UsedMemory = parseIntFromInfo(memInfo, "used_memory")

	// Get client connection count
	clientInfo, _ := r.rdb.Info(ctx, "clients").Result()
	metrics.ConnectedClients = int(parseIntFromInfo(clientInfo, "connected_clients"))

	// Calculate hit rate
	keyspaceHits := parseIntFromInfo(info, "keyspace_hits")
	keyspaceMisses := parseIntFromInfo(info, "keyspace_misses")
	if total := keyspaceHits + keyspaceMisses; total > 0 {
		metrics.HitRate = float64(keyspaceHits) / float64(total) * 100
	}

	return metrics, nil
}
