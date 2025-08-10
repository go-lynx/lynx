package redis

import (
 "time"
	"github.com/redis/go-redis/v9"
)

// startPoolStatsCollector 周期性抓取 PoolStats 并上报 Prometheus
func (r *PlugRedis) startPoolStatsCollector() {
	r.statsQuit = make(chan struct{})
	r.statsWG.Add(1)
	go func() {
		defer r.statsWG.Done()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-r.statsQuit:
				return
			case <-ticker.C:
				r.observePoolStats()
			}
		}
	}()
	// 立即采集一次
	r.observePoolStats()
}

func (r *PlugRedis) observePoolStats() {
	if r.rdb == nil {
		return
	}
	// 兼容不同客户端类型
	switch c := r.rdb.(type) {
	case *redis.Client:
		ps := c.PoolStats()
		r.setPoolStats(ps)
	case *redis.ClusterClient:
		ps := c.PoolStats()
		r.setPoolStats(ps)
	case *redis.Ring:
		ps := c.PoolStats()
		r.setPoolStats(ps)
	default:
		// 尝试通过接口断言（某些版本 UniversalClient 可能直接实现 PoolStats 方法）
		type poolStater interface{ PoolStats() *redis.PoolStats }
		if pc, ok := any(r.rdb).(poolStater); ok {
			r.setPoolStats(pc.PoolStats())
		}
	}
}

func (r *PlugRedis) setPoolStats(ps *redis.PoolStats) {
	if ps == nil {
		return
	}
	redisPoolHits.Set(float64(ps.Hits))
	redisPoolMisses.Set(float64(ps.Misses))
	redisPoolTimeouts.Set(float64(ps.Timeouts))
	redisPoolTotalConns.Set(float64(ps.TotalConns))
	redisPoolIdleConns.Set(float64(ps.IdleConns))
	redisPoolStaleConns.Set(float64(ps.StaleConns))
}
