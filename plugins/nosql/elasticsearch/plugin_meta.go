package elasticsearch

import (
	"github.com/go-lynx/lynx/plugins"
)

// Plugin metadata
const (
	// Plugin unique name
	pluginName = "elasticsearch.client"
	// Plugin version number
	pluginVersion = "v1.0.0"
	// Plugin description
	pluginDescription = "elasticsearch plugin for lynx framework"
	// Configuration prefix, used to read plugin-related configuration from config
	confPrefix = "lynx.elasticsearch"
)

// NewElasticsearchClient creates a new Elasticsearch plugin instance
// Returns a pointer to PlugElasticsearch struct
func NewElasticsearchClient() *PlugElasticsearch {
	return &PlugElasticsearch{
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
