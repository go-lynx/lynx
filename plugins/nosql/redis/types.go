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
