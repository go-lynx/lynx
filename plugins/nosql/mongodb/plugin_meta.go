package mongodb

import (
	"github.com/go-lynx/lynx/plugins"
)

// Plugin metadata
const (
	// Plugin unique name
	pluginName = "mongodb.client"
	// Plugin version number
	pluginVersion = "v1.0.0"
	// Plugin description
	pluginDescription = "mongodb plugin for lynx framework"
	// Configuration prefix, used to read plugin-related configuration from config
	confPrefix = "lynx.mongodb"
)

// NewMongoDBClient creates a new MongoDB plugin instance
// Returns a pointer to PlugMongoDB struct
func NewMongoDBClient() *PlugMongoDB {
	return &PlugMongoDB{
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
