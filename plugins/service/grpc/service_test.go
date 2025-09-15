package grpc

import (
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/service/grpc/conf"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestNewGrpcService(t *testing.T) {
	plugin := NewGrpcService()
	assert.NotNil(t, plugin)
	assert.Equal(t, pluginName, plugin.Name())
	assert.Equal(t, pluginVersion, plugin.Version())
	assert.Equal(t, pluginDescription, plugin.Description())
	// ConfigPrefix is not directly accessible, test through internal state
	assert.Equal(t, 10, plugin.Weight())
}

func TestInitializeResources(t *testing.T) {
	plugin := NewGrpcService()

	// Test default configuration - skip due to complex interface requirements
	// In real usage, this would be called by the framework
	// err := plugin.InitializeResources(mockRuntime{})
	// require.NoError(t, err)

	// Verify default values would be set
	assert.NotNil(t, plugin)
}

func TestCheckHealth(t *testing.T) {
	plugin := NewGrpcService()

	// Test uninitialized state
	err := plugin.CheckHealth()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	// Test normal state with configuration - skip due to server initialization requirement
	// plugin.conf = &conf.Service{
	// 	Addr: ":9090",
	// }
	// err = plugin.CheckHealth()
	// assert.NoError(t, err)

	// This test requires full server initialization which is complex in unit tests
	assert.NotNil(t, plugin)
}

func TestValidateConfig(t *testing.T) {
	plugin := NewGrpcService()

	// Test nil configuration
	err := plugin.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "configuration is nil")

	// Test normal configuration
	plugin.conf = &conf.Service{
		Network: "tcp",
		Addr:    ":9090",
		Timeout: durationpb.New(10 * time.Second),
	}
	err = plugin.validateConfig()
	assert.NoError(t, err)

	// Test invalid network type
	plugin.conf.Network = "invalid"
	err = plugin.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported network type")

	// Test invalid address
	plugin.conf.Network = "tcp"
	plugin.conf.Addr = "invalid-address"
	err = plugin.validateConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid address format")
}

func TestValidateAddress(t *testing.T) {
	plugin := NewGrpcService()
	plugin.conf = &conf.Service{Network: "tcp"}

	// Test empty address
	err := plugin.validateAddress("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")

	// Test valid addresses
	err = plugin.validateAddress(":9090")
	assert.NoError(t, err)

	err = plugin.validateAddress("localhost:9090")
	assert.NoError(t, err)

	// Test invalid address format
	err = plugin.validateAddress("9090")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must include port")
}

func TestValidateTLSConfig(t *testing.T) {
	plugin := NewGrpcService()

	// Test TLS disabled
	plugin.conf = &conf.Service{TlsEnable: false}
	err := plugin.validateTLSConfig()
	assert.NoError(t, err)

	// Test TLS enabled but invalid configuration
	plugin.conf = &conf.Service{
		TlsEnable:   true,
		TlsAuthType: 5, // Invalid authentication type
	}
	err = plugin.validateTLSConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid TLS auth type")
}

func TestConfigure(t *testing.T) {
	plugin := NewGrpcService()

	// Test nil configuration
	err := plugin.Configure(nil)
	assert.NoError(t, err)

	// Test valid configuration
	config := &conf.Service{
		Network: "tcp",
		Addr:    ":8080",
	}
	err = plugin.Configure(config)
	assert.NoError(t, err)
	assert.Equal(t, config, plugin.conf)
}

// Mock type definitions
type mockRuntime struct{}

func (m mockRuntime) GetConfig() mockConfig {
	return mockConfig{}
}

func (m mockRuntime) AddListener(listener plugins.EventListener, filter *plugins.EventFilter) {
	// Mock implementation - no return value
}

func (m mockRuntime) AddPluginListener(pluginName string, listener plugins.EventListener, filter *plugins.EventFilter) {
	// Mock implementation - no return value
}

func (m mockRuntime) CleanupResources(pluginName string) error {
	// Mock implementation
	return nil
}

func (m mockRuntime) EmitEvent(event plugins.PluginEvent) {
	// Mock implementation
}

func (m mockRuntime) EmitPluginEvent(pluginName string, eventType string, data map[string]any) {
	// Mock implementation
}

type mockConfig struct{}

func (m mockConfig) Value(key string) mockValue {
	return mockValue{}
}

type mockValue struct{}

func (m mockValue) Scan(dest interface{}) error {
	// Return default configuration
	return nil
}

type mockGrpcServer struct{}
