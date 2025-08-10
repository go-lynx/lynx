package redis

import (
	"github.com/go-lynx/lynx/plugins"
)

// 插件元数据
const (
	// 插件唯一名称
	pluginName = "redis.client"
	// 插件版本号
	pluginVersion = "v2.0.0"
	// 插件描述信息
	pluginDescription = "redis plugin for lynx framework"
	// 配置前缀，用于从配置中读取插件相关配置
	confPrefix = "lynx.redis"
)

// NewRedisClient 创建一个新的 Redis 插件实例
// 返回一个指向 PlugRedis 结构体的指针
func NewRedisClient() *PlugRedis {
	return &PlugRedis{
		BasePlugin: plugins.NewBasePlugin(
			// 生成插件唯一 ID
			plugins.GeneratePluginID("", pluginName, pluginVersion),
			// 插件名称
			pluginName,
			// 插件描述
			pluginDescription,
			// 插件版本
			pluginVersion,
			// 配置前缀
			confPrefix,
			// 权重
			100,
		),
	}
}
