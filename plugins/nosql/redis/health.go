package redis

import (
	"context"
	"strings"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/redis/go-redis/v9"
)

// detectMode 返回 single/cluster/sentinel
func (r *PlugRedis) detectMode() string {
	if r.conf.Sentinel != nil && r.conf.Sentinel.MasterName != "" {
		return "sentinel"
	}
	addrs := r.currentAddrs()
	if len(addrs) > 1 {
		return "cluster"
	}
	return "single"
}

func (r *PlugRedis) currentAddrs() []string {
	if r.conf.Sentinel != nil && len(r.conf.Sentinel.Addrs) > 0 {
		return append([]string{}, r.conf.Sentinel.Addrs...)
	}
	if len(r.conf.Addrs) > 0 {
		return append([]string{}, r.conf.Addrs...)
	}
	return nil
}

// enhancedReadinessCheck 基于模式做更严格的就绪检查
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
		// 单机/哨兵：读取 INFO replication 判定角色
		info, err := r.rdb.Info(ctx, "replication").Result()
		if err == nil {
			if strings.Contains(info, "role:master") {
				redisIsMaster.Set(1)
			} else {
				redisIsMaster.Set(0)
			}
			// 解析 connected_slaves:N
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
	// 写一次 server_info 指标
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

// startInfoCollector 周期性采集 INFO 与 cluster 状态
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
