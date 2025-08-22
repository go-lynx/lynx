package seata

import (
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/seata/conf"
	"github.com/seata/seata-go/pkg/client"
)

// Plugin metadata
const (
	// pluginName is the unique identifier for the HTTP server plugin, used to identify the plugin in the plugin system.
	pluginName = "seata.server"

	// pluginVersion indicates the current version of the HTTP server plugin.
	pluginVersion = "v2.0.0"

	// pluginDescription briefly describes the functionality of the HTTP server plugin.
	pluginDescription = "seata transaction server plugin for Lynx framework"

	// confPrefix is the configuration prefix used when loading HTTP server configuration.
	confPrefix = "lynx.seata"
)

type TxSeataClient struct {
	// Embed base plugin, inherit common properties and methods of the plugin
	*plugins.BasePlugin
	// HTTP server configuration information
	conf *conf.Seata
}

// NewTxSeataClient creates a new HTTP server plugin instance.
// This function initializes the basic information of the plugin and returns a pointer to the ServiceHttp structure.
func NewTxSeataClient() *TxSeataClient {
	return &TxSeataClient{
		BasePlugin: plugins.NewBasePlugin(
			// Generate unique plugin ID
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
			90,
		),
	}
}

// InitializeResources method is used to load and initialize the Seata plugin
func (t *TxSeataClient) InitializeResources(rt plugins.Runtime) error {
	// Initialize an empty configuration structure
	t.conf = &conf.Seata{}

	// Scan and load Seata configuration from runtime configuration
	err := rt.GetConfig().Value(confPrefix).Scan(t.conf)
	if err != nil {
		return err
	}

	// Set default configuration
	defaultConf := &conf.Seata{
		// Default configuration file path is ./conf/seata.yml
		ConfigFilePath: "./conf/seata.yml",
	}

	// Use default values for unset fields
	if t.conf.ConfigFilePath == "" {
		t.conf.ConfigFilePath = defaultConf.ConfigFilePath
	}

	return nil
}

func (t *TxSeataClient) StartupTasks() error {
	// Use Lynx application's Helper to log Seata plugin initialization information
	log.Infof("Initializing seata")
	// If the Seata plugin is enabled, initialize the Seata client
	if t.conf.GetEnabled() {
		// Call client.InitPath method to initialize the Seata client, using the path from configuration
		client.InitPath(t.conf.GetConfigFilePath())
	}
	// Use Lynx application's Helper to log successful Seata service initialization
	log.Infof("seata successfully initialized")
	// Return Seata plugin instance and nil error, indicating successful loading
	return nil
}

func (t *TxSeataClient) CleanupTasks() error {
	return nil
}
