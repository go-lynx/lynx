package polaris

import (
	"fmt"

	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-lynx/lynx/app"
	"github.com/polarismesh/polaris-go/pkg/model"
)

var plugin *PlugPolaris

// GetPolarisPlugin 获取 Polaris 插件实例
func GetPolarisPlugin() (*PlugPolaris, error) {
	if plugin == nil {
		return nil, fmt.Errorf("polaris plugin not found")
	}
	return plugin, nil
}

// GetServiceInstances 获取服务实例
func GetServiceInstances(serviceName string) ([]model.Instance, error) {
	if plugin == nil {
		return nil, fmt.Errorf("polaris plugin not found")
	}
	return plugin.GetServiceInstances(serviceName)
}

// GetConfig 获取配置
// 全局 API：通过文件名和组名获取配置内容
func GetConfig(fileName, group string) (string, error) {
	if plugin == nil {
		return "", fmt.Errorf("polaris plugin not found")
	}
	return plugin.GetConfigValue(fileName, group)
}

// WatchService 监听服务变更
// 全局 API：监听指定服务的变更事件
func WatchService(serviceName string) (*ServiceWatcher, error) {
	if plugin == nil {
		return nil, fmt.Errorf("polaris plugin not found")
	}
	return plugin.WatchService(serviceName)
}

// WatchConfig 监听配置变更
// 全局 API：监听指定配置的变更事件
func WatchConfig(fileName, group string) (*ConfigWatcher, error) {
	if plugin == nil {
		return nil, fmt.Errorf("polaris plugin not found")
	}
	return plugin.WatchConfig(fileName, group)
}

// CheckRateLimit 检查限流
// 全局 API：检查指定服务的限流状态
func CheckRateLimit(serviceName string, labels map[string]string) (bool, error) {
	if plugin == nil {
		return false, fmt.Errorf("polaris plugin not found")
	}
	return plugin.CheckRateLimit(serviceName, labels)
}

// GetMetrics 获取监控指标
// 全局 API：获取插件的监控指标
func GetMetrics() *Metrics {
	if plugin == nil {
		return nil
	}
	return plugin.GetMetrics()
}

// IsHealthy 检查插件健康状态
// 全局 API：检查插件是否健康
func IsHealthy() error {
	if plugin == nil {
		return fmt.Errorf("polaris plugin not found")
	}
	return plugin.CheckHealth()
}

// GetPolaris 函数用于从应用的插件管理器中获取 Polaris 实例。
// 该实例可用于与 Polaris 服务进行交互，如服务发现、配置管理等。
// 返回值为 *polaris.Polaris 类型的指针，指向获取到的 Polaris 实例。
func GetPolaris() *polaris.Polaris {
	// 从应用的插件管理器中获取指定名称的插件实例，
	// 并将其类型断言为 *PlugPolaris，然后返回其内部的 polaris 字段。
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil
	}
	return plugin.(*PlugPolaris).polaris
}

// GetPlugin 函数用于从应用的插件管理器中获取 PlugPolaris 插件实例。
// 该实例可用于调用插件提供的各种方法。
// 返回值为 *PlugPolaris 类型的指针，指向获取到的插件实例。
func GetPlugin() *PlugPolaris {
	// 从应用的插件管理器中获取指定名称的插件实例，
	// 并将其类型断言为 *PlugPolaris 后返回。
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil
	}
	return plugin.(*PlugPolaris)
}
