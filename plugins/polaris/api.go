package polaris

import (
	"fmt"

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
