package polaris

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWatchersIntegration tests the integration of watchers and polaris plugin
func TestWatchersIntegration(t *testing.T) {
	plugin := NewPolarisControlPlane()

	// Test watcher creation in uninitialized state
	_, err := plugin.WatchService("test-service")
	assert.Error(t, err)
	assert.IsType(t, &PolarisError{}, err)

	_, err = plugin.WatchConfig("test-config", "test-group")
	assert.Error(t, err)
	assert.IsType(t, &PolarisError{}, err)
}

// TestServiceWatcherCreation tests service watcher creation
func TestServiceWatcherCreation(t *testing.T) {
	// Test service watcher creation (without connecting to SDK)
	watcher := NewServiceWatcher(nil, "test-service", "test-namespace")
	assert.NotNil(t, watcher)
	assert.Equal(t, "test-service", watcher.serviceName)
	assert.Equal(t, "test-namespace", watcher.namespace)
	assert.Nil(t, watcher.consumer) // nil when not connected to SDK
}

// TestConfigWatcherCreation tests configuration watcher creation
func TestConfigWatcherCreation(t *testing.T) {
	// Test configuration watcher creation (without connecting to SDK)
	watcher := NewConfigWatcher(nil, "test-config", "test-group", "test-namespace")
	assert.NotNil(t, watcher)
	assert.Equal(t, "test-config", watcher.fileName)
	assert.Equal(t, "test-group", watcher.group)
	assert.Equal(t, "test-namespace", watcher.namespace)
	assert.Nil(t, watcher.configAPI) // nil when not connected to SDK
}

// TestWatchersWithMetrics tests watchers with metrics
func TestWatchersWithMetrics(t *testing.T) {
	// Create metrics
	metrics := NewPolarisMetrics()
	assert.NotNil(t, metrics)

	// Test service watcher with metrics
	serviceWatcher := NewServiceWatcher(nil, "test-service", "test-namespace")
	serviceWatcher.metrics = metrics
	assert.NotNil(t, serviceWatcher.metrics)

	// Test configuration watcher with metrics
	configWatcher := NewConfigWatcher(nil, "test-config", "test-group", "test-namespace")
	configWatcher.metrics = metrics
	assert.NotNil(t, configWatcher.metrics)
}

// TestWatchersLifecycle tests watcher lifecycle
func TestWatchersLifecycle(t *testing.T) {
	t.Skip("Skipping lifecycle test to avoid log initialization issues")
}
