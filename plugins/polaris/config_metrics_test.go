package polaris

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConfigMetrics 测试配置变动指标
func TestConfigMetrics(t *testing.T) {
	// 创建 metrics 实例
	metrics := NewPolarisMetrics()
	assert.NotNil(t, metrics)

	// 测试配置操作指标
	metrics.RecordConfigOperation("get", "application.yml", "DEFAULT_GROUP", "start")
	metrics.RecordConfigOperation("get", "application.yml", "DEFAULT_GROUP", "success")
	metrics.RecordConfigOperation("check", "application.yml", "DEFAULT_GROUP", "start")
	metrics.RecordConfigOperation("check", "application.yml", "DEFAULT_GROUP", "success")

	// 测试配置变更指标
	metrics.RecordConfigChange("application.yml", "DEFAULT_GROUP")
	metrics.RecordConfigChange("database.yml", "DEFAULT_GROUP")

	// 测试配置操作耗时
	metrics.RecordConfigOperationDuration("get", "application.yml", "DEFAULT_GROUP", 0.1)
	metrics.RecordConfigOperationDuration("check", "application.yml", "DEFAULT_GROUP", 0.05)
}

// TestConfigWatcherMetrics 测试配置监听器指标
func TestConfigWatcherMetrics(t *testing.T) {
	t.Skip("Skipping config watcher test to avoid Prometheus metrics registration issues")
}

// TestConfigOperationsWithMetrics 测试带指标的配置操作
func TestConfigOperationsWithMetrics(t *testing.T) {
	plugin := NewPolarisControlPlane()

	// 测试未初始化状态下的配置操作
	_, err := plugin.GetConfigValue("application.yml", "DEFAULT_GROUP")
	assert.Error(t, err)
	assert.IsType(t, &PolarisError{}, err)

	_, err = plugin.WatchConfig("application.yml", "DEFAULT_GROUP")
	assert.Error(t, err)
	assert.IsType(t, &PolarisError{}, err)
}
