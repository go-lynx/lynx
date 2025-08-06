package polaris

import (
	"fmt"

	"github.com/polarismesh/polaris-go/pkg/model"
)

var plugin *PlugPolaris

func init() {
	// 插件注册由框架处理
}

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
func GetConfig(fileName, group string) (string, error) {
	if plugin == nil {
		return "", fmt.Errorf("polaris plugin not found")
	}
	return plugin.GetConfigValue(fileName, group)
}

// WatchService 监听服务变更
func WatchService(serviceName string) (*ServiceWatcher, error) {
	if plugin == nil {
		return nil, fmt.Errorf("polaris plugin not found")
	}
	return plugin.WatchService(serviceName)
}

// CheckRateLimit 检查限流
func CheckRateLimit(serviceName string, labels map[string]string) (bool, error) {
	if plugin == nil {
		return false, fmt.Errorf("polaris plugin not found")
	}
	return plugin.CheckRateLimit(serviceName, labels)
}
