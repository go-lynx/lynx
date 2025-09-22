package sentinel

import (
	"github.com/go-lynx/lynx/plugins"
)

// Plugin metadata
const (
	// Plugin unique name
	pluginName = "sentinel.flow_control"
	// Alias for external access
	PluginName = pluginName
	// Plugin version number
	pluginVersion = "v1.0.0"
	// Alias for external access
	PluginVersion = pluginVersion
	// Plugin description
	pluginDescription = "Sentinel flow control and circuit breaker plugin for lynx framework"
	// Alias for external access
	PluginDescription = pluginDescription
	// Configuration prefix, used to read plugin-related configuration from config
	confPrefix = "lynx.sentinel"
	// Alias for compatibility
	ConfPrefix = "lynx.sentinel"
	// Plugin weight - higher values load first, Sentinel should load early for protection
	pluginWeight = 200
	// Alias for external access
	PluginWeight = pluginWeight
)

// NewSentinelPlugin creates a new Sentinel plugin instance
// Returns a pointer to PlugSentinel struct
func NewSentinelPlugin() *PlugSentinel {
	return &PlugSentinel{
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
			// Weight - high priority for protection
			pluginWeight,
		),
		stopCh: make(chan struct{}),
	}
}