package elasticsearch

import (
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
)

// init function is a special function in Go that is automatically executed when the package is loaded.
// This function registers the Elasticsearch client plugin to the global plugin factory.
func init() {
	// Call the RegisterPlugin method of the global plugin factory to register the plugin.
	// The first parameter pluginName is the unique name of the plugin, used to identify the plugin.
	// The second parameter confPrefix is the configuration prefix, used to read plugin-related configuration from the config.
	// The third parameter is an anonymous function that returns an instance of plugins.Plugin interface type,
	// by calling the NewElasticsearchClient function to create a new Elasticsearch client plugin instance.
	factory.GlobalTypedFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		return NewElasticsearchClient()
	})
}

// GetElasticsearch function is used to get the Elasticsearch client instance.
// It gets the plugin manager through the global Lynx application instance, then gets the corresponding plugin instance by plugin name,
// finally converts the plugin instance to *PlugElasticsearch type and returns its client field, which is the Elasticsearch client.
func GetElasticsearch() *elasticsearch.Client {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil
	}
	return plugin.(*PlugElasticsearch).GetClient()
}

// GetElasticsearchPlugin gets the Elasticsearch plugin instance
func GetElasticsearchPlugin() *PlugElasticsearch {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil
	}
	return plugin.(*PlugElasticsearch)
}
