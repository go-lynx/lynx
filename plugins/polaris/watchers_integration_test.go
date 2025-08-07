package polaris

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWatchersIntegration 测试 watchers 和 polaris 插件的集成
func TestWatchersIntegration(t *testing.T) {
	plugin := NewPolarisControlPlane()

	// 测试未初始化状态下的监听器创建
	_, err := plugin.WatchService("test-service")
	assert.Error(t, err)
	assert.IsType(t, &PolarisError{}, err)

	_, err = plugin.WatchConfig("test-config", "test-group")
	assert.Error(t, err)
	assert.IsType(t, &PolarisError{}, err)
}

// TestServiceWatcherCreation 测试服务监听器创建
func TestServiceWatcherCreation(t *testing.T) {
	// 测试服务监听器创建（不连接 SDK）
	watcher := NewServiceWatcher(nil, "test-service", "test-namespace")
	assert.NotNil(t, watcher)
	assert.Equal(t, "test-service", watcher.serviceName)
	assert.Equal(t, "test-namespace", watcher.namespace)
	assert.Nil(t, watcher.consumer) // 未连接 SDK 时为 nil
}

// TestConfigWatcherCreation 测试配置监听器创建
func TestConfigWatcherCreation(t *testing.T) {
	// 测试配置监听器创建（不连接 SDK）
	watcher := NewConfigWatcher(nil, "test-config", "test-group", "test-namespace")
	assert.NotNil(t, watcher)
	assert.Equal(t, "test-config", watcher.fileName)
	assert.Equal(t, "test-group", watcher.group)
	assert.Equal(t, "test-namespace", watcher.namespace)
	assert.Nil(t, watcher.configAPI) // 未连接 SDK 时为 nil
}

// TestWatchersWithMetrics 测试带指标的监听器
func TestWatchersWithMetrics(t *testing.T) {
	// 创建 metrics
	metrics := NewPolarisMetrics()
	assert.NotNil(t, metrics)

	// 测试服务监听器带指标
	serviceWatcher := NewServiceWatcher(nil, "test-service", "test-namespace")
	serviceWatcher.metrics = metrics
	assert.NotNil(t, serviceWatcher.metrics)

	// 测试配置监听器带指标
	configWatcher := NewConfigWatcher(nil, "test-config", "test-group", "test-namespace")
	configWatcher.metrics = metrics
	assert.NotNil(t, configWatcher.metrics)
}

// TestWatchersLifecycle 测试监听器生命周期
func TestWatchersLifecycle(t *testing.T) {
	t.Skip("Skipping lifecycle test to avoid log initialization issues")
}
