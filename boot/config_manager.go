package boot

import (
	"os"
	"sync"
)

// ConfigManager manages application configuration paths
type ConfigManager struct {
	configPath string
	mu         sync.RWMutex
}

var (
	configManager *ConfigManager
	once          sync.Once
)

// GetConfigManager returns singleton configuration manager instance
func GetConfigManager() *ConfigManager {
	once.Do(func() {
		configManager = &ConfigManager{}
	})
	return configManager
}

// SetConfigPath sets configuration path
func (cm *ConfigManager) SetConfigPath(path string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.configPath = path
}

// GetConfigPath gets configuration path
func (cm *ConfigManager) GetConfigPath() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.configPath
}

// GetDefaultConfigPath gets default configuration path
func (cm *ConfigManager) GetDefaultConfigPath() string {
	// Prioritize environment variable
	if envPath := os.Getenv("LYNX_CONFIG_PATH"); envPath != "" {
		return envPath
	}
	// Default to configs directory under current directory
	return "./configs"
}

// IsConfigPathSet checks if configuration path is set
func (cm *ConfigManager) IsConfigPathSet() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.configPath != ""
}
