package boot

import (
	"os"
	"sync"
)

// ConfigManager 管理应用程序的配置路径
type ConfigManager struct {
	configPath string
	mu         sync.RWMutex
}

var (
	configManager *ConfigManager
	once          sync.Once
)

// GetConfigManager 返回单例的配置管理器实例
func GetConfigManager() *ConfigManager {
	once.Do(func() {
		configManager = &ConfigManager{}
	})
	return configManager
}

// SetConfigPath 设置配置路径
func (cm *ConfigManager) SetConfigPath(path string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.configPath = path
}

// GetConfigPath 获取配置路径
func (cm *ConfigManager) GetConfigPath() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.configPath
}

// GetDefaultConfigPath 获取默认配置路径
func (cm *ConfigManager) GetDefaultConfigPath() string {
	// 优先使用环境变量
	if envPath := os.Getenv("LYNX_CONFIG_PATH"); envPath != "" {
		return envPath
	}
	// 默认使用当前目录下的configs
	return "./configs"
}

// IsConfigPathSet 检查配置路径是否已设置
func (cm *ConfigManager) IsConfigPathSet() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.configPath != ""
}
