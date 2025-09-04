package redis

import (
	"github.com/go-lynx/lynx/plugins"
)

// Plugin metadata
const (
	// Plugin unique name
	pluginName = "redis.client"
	// Plugin version number
	pluginVersion = "v2.0.0"
	// Plugin description
	pluginDescription = "redis plugin for lynx framework"
	// Configuration prefix, used to read plugin-related configuration from config
	confPrefix = "lynx.redis"
)

// NewRedisClient creates a new Redis plugin instance
// Returns a pointer to PlugRedis struct
func NewRedisClient() *PlugRedis {
	return &PlugRedis{
		BasePlugin: plugins.NewBasePlugin(
			// Generate plugin unique ID
			plugins.GeneratePluginID("", pluginName, pluginVersion),
			// Plugin name
			pluginName,
			// Plugin description
			pluginDescription,
			// Plugin version
			pluginVersion,
			// Configuration prefix
			confPrefix,
			// Weight
			100,
		),
	}
}
