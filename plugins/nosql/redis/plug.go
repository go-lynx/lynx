package redis

import (
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
	"github.com/redis/go-redis/v9"
)

// init function is a special function in Go that is automatically executed when the package is loaded.
// This function registers the Redis client plugin to the global plugin factory.
func init() {
	// Register the Redis client plugin to the global plugin factory.
	// The first parameter pluginName is the unique plugin name used for identification.
	// The second parameter confPrefix is the configuration prefix, used to read plugin-related configuration from the config.
	// The third parameter is an anonymous function that returns an instance of plugins.Plugin interface type,
	// by calling the NewRedisClient function to create a new Redis client plugin instance.
	factory.GlobalTypedFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		return NewRedisClient()
	})
}

// GetRedis function is used to get the Redis client instance.
// It gets the plugin manager through the global Lynx application instance, then gets the corresponding plugin instance by plugin name,
// finally converts the plugin instance to *PlugRedis type and returns its rdb field, which is the Redis client.
// GetRedis returns the underlying *redis.Client, only available in single node mode; returns nil for Cluster/Sentinel.
func GetRedis() *redis.Client {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil
	}
	// Try to assert as *redis.Client (single node)
	if c, ok := plugin.(*PlugRedis).rdb.(*redis.Client); ok {
		return c
	}
	return nil
}

// GetUniversalRedis returns the universal client, usable for single node/cluster/sentinel modes.
func GetUniversalRedis() redis.UniversalClient {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil
	}
	return plugin.(*PlugRedis).rdb
}
