package redis

import (
	"sync"

	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/nosql/redis/conf"
	"github.com/redis/go-redis/v9"
)

// PlugRedis 表示 Redis 插件实例
type PlugRedis struct {
	// 继承基础插件
	*plugins.BasePlugin
	// Redis 配置
	conf *conf.Redis
	// Redis 客户端实例（支持单机/集群/哨兵）
	rdb redis.UniversalClient
	// 指标采集
	statsQuit chan struct{}
	statsWG   sync.WaitGroup
}
