package polaris

import (
	"fmt"

	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-lynx/lynx/app"
	"github.com/polarismesh/polaris-go/pkg/model"
)

var plugin *PlugPolaris

// GetPolarisPlugin returns the Polaris plugin instance.
func GetPolarisPlugin() (*PlugPolaris, error) {
	if plugin == nil {
		return nil, fmt.Errorf("polaris plugin not found")
	}
	return plugin, nil
}

// GetServiceInstances returns service instances.
func GetServiceInstances(serviceName string) ([]model.Instance, error) {
	if plugin == nil {
		return nil, fmt.Errorf("polaris plugin not found")
	}
	return plugin.GetServiceInstances(serviceName)
}

// GetConfig fetches configuration by file name and group.
// Global API: retrieve config content by file name and group.
func GetConfig(fileName, group string) (string, error) {
	if plugin == nil {
		return "", fmt.Errorf("polaris plugin not found")
	}
	return plugin.GetConfigValue(fileName, group)
}

// WatchService watches service changes.
// Global API: watch change events of the specified service.
func WatchService(serviceName string) (*ServiceWatcher, error) {
	if plugin == nil {
		return nil, fmt.Errorf("polaris plugin not found")
	}
	return plugin.WatchService(serviceName)
}

// WatchConfig watches configuration changes.
// Global API: watch change events of the specified configuration.
func WatchConfig(fileName, group string) (*ConfigWatcher, error) {
	if plugin == nil {
		return nil, fmt.Errorf("polaris plugin not found")
	}
	return plugin.WatchConfig(fileName, group)
}

// CheckRateLimit checks rate limit status for a service.
// Global API: check the rate limit status of the specified service.
func CheckRateLimit(serviceName string, labels map[string]string) (bool, error) {
	if plugin == nil {
		return false, fmt.Errorf("polaris plugin not found")
	}
	return plugin.CheckRateLimit(serviceName, labels)
}

// GetMetrics returns plugin metrics.
// Global API: get metrics exposed by the plugin.
func GetMetrics() *Metrics {
	if plugin == nil {
		return nil
	}
	return plugin.GetMetrics()
}

// IsHealthy checks plugin health status.
// Global API: verify whether the plugin is healthy.
func IsHealthy() error {
	if plugin == nil {
		return fmt.Errorf("polaris plugin not found")
	}
	return plugin.CheckHealth()
}

// GetPolaris obtains the Polaris instance from the application's plugin manager.
// The instance can be used to interact with Polaris services (service discovery, config management, etc.).
// It returns a *polaris.Polaris pointing to the instance.
func GetPolaris() *polaris.Polaris {
	// Fetch the plugin by name from the application's plugin manager,
	// cast it to *PlugPolaris and return its internal `polaris` field.
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil
	}
	return plugin.(*PlugPolaris).polaris
}

// GetPlugin obtains the PlugPolaris plugin instance from the application's plugin manager.
// The instance can be used to invoke methods provided by the plugin.
// It returns a *PlugPolaris pointing to the instance.
func GetPlugin() *PlugPolaris {
	// Fetch the plugin by name from the application's plugin manager,
	// cast it to *PlugPolaris and return it.
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil
	}
	return plugin.(*PlugPolaris)
}
