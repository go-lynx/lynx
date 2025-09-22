package redis

import (
	"sync"

	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/nosql/redis/conf"
	"github.com/redis/go-redis/v9"
)

// PlugRedis represents a Redis plugin instance
type PlugRedis struct {
	// Inherits from base plugin
	*plugins.BasePlugin
	// Redis configuration
	conf *conf.Redis
	// Redis client instance (supports single node/cluster/sentinel)
	rdb redis.UniversalClient
	// Metrics collection
	statsQuit chan struct{}
	statsWG   sync.WaitGroup
}

// GetPoolStats returns the connection pool statistics
func (r *PlugRedis) GetPoolStats() *redis.PoolStats {
	if r.rdb == nil {
		return nil
	}
	
	// Compatible with different client types
	switch c := r.rdb.(type) {
	case *redis.Client:
		return c.PoolStats()
	case *redis.ClusterClient:
		return c.PoolStats()
	case *redis.Ring:
		return c.PoolStats()
	default:
		// Try interface assertion (some versions of UniversalClient may directly implement PoolStats method)
		type poolStater interface{ PoolStats() *redis.PoolStats }
		if pc, ok := any(r.rdb).(poolStater); ok {
			return pc.PoolStats()
		}
	}
	return nil
}

// HealthCheck performs a health check on the Redis connection
func (r *PlugRedis) HealthCheck() (bool, error) {
	err := r.CheckHealth()
	return err == nil, err
}
