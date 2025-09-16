package grpc

import (
	"net"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/grpc"
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

	// Test with mock runtime
	mockRT := &mockRuntime{}
	err := plugin.InitializeResources(mockRT)
	assert.NoError(t, err)

	// Verify plugin is properly initialized
	assert.NotNil(t, plugin)
}

func TestCheckHealth(t *testing.T) {
	plugin := NewGrpcService()

	// Test uninitialized state
	err := plugin.CheckHealth()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	// Test with configuration but no server
	plugin.conf = &conf.Service{
		Addr: ":9090",
	}
	err = plugin.CheckHealth()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server not running")

	// Test with mock server running
	plugin.server = grpc.NewServer()
	err = plugin.CheckHealth()
	assert.NoError(t, err)
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

func (m mockRuntime) GetConfig() config.Config {
	return &mockConfig{}
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

func (m mockRuntime) GetCurrentPluginContext() string { return "" }
func (m mockRuntime) GetEventHistory(filter plugins.EventFilter) []plugins.PluginEvent { return nil }
func (m mockRuntime) GetEventStats() map[string]any { return nil }
func (m mockRuntime) GetLogger() log.Logger { return log.DefaultLogger }
func (m mockRuntime) GetPluginEventHistory(pluginName string, filter plugins.EventFilter) []plugins.PluginEvent { return nil }
func (m mockRuntime) GetResourceStats() map[string]any { return nil }
func (m mockRuntime) GetSharedResource(name string) (any, error) { return nil, nil }
func (m mockRuntime) GetPrivateResource(name string) (any, error) { return nil, nil }
func (m mockRuntime) GetResource(name string) (any, error) { return nil, nil }
func (m mockRuntime) GetResourceInfo(name string) (*plugins.ResourceInfo, error) { return nil, nil }
func (m mockRuntime) ListResources() []*plugins.ResourceInfo { return nil }
func (m mockRuntime) RegisterPrivateResource(name string, resource any) error { return nil }
func (m mockRuntime) RegisterResource(name string, resource any) error { return nil }
func (m mockRuntime) RegisterSharedResource(name string, resource any) error { return nil }
func (m mockRuntime) RemoveListener(listener plugins.EventListener) {}
func (m mockRuntime) RemovePluginListener(pluginName string, listener plugins.EventListener) {}
func (m mockRuntime) SetConfig(conf config.Config) {}
func (m mockRuntime) SetEventDispatchMode(mode string) error { return nil }
func (m mockRuntime) UnregisterPrivateResource(name string) error { return nil }
func (m mockRuntime) UnregisterResource(name string) error { return nil }
func (m mockRuntime) UnregisterSharedResource(name string) error { return nil }

type mockConfig struct{}

func (m *mockConfig) Value(key string) config.Value {
	return &mockValue{}
}

func (m *mockConfig) Load() error { return nil }
func (m *mockConfig) Watch(key string, o config.Observer) error { return nil }
func (m *mockConfig) Close() error { return nil }
func (m *mockConfig) Scan(dest interface{}) error { return nil }

type mockValue struct{}

func (m *mockValue) Scan(dest interface{}) error {
	if service, ok := dest.(*conf.Service); ok {
		service.Addr = ":9090"
		service.Network = "tcp"
		service.Timeout = &durationpb.Duration{Seconds: 30}
	}
	return nil
}

func (m *mockValue) Bool() (bool, error) { return false, nil }
func (m *mockValue) Int() (int64, error) { return 0, nil }
func (m *mockValue) Float() (float64, error) { return 0, nil }
func (m *mockValue) String() (string, error) { return "", nil }
func (m *mockValue) Duration() (time.Duration, error) { return 0, nil }
func (m *mockValue) Slice() ([]config.Value, error) { return nil, nil }
func (m *mockValue) Map() (map[string]config.Value, error) { return nil, nil }
func (m *mockValue) Load() any { return nil }
func (m *mockValue) Store(any) {}

type mockGrpcServer struct {
	*grpc.Server
}

func (m *mockGrpcServer) Serve(lis net.Listener) error {
	return nil
}

func (m *mockGrpcServer) Stop() {
	// Mock implementation
}

func (m *mockGrpcServer) GracefulStop() {
	// Mock graceful stop implementation
}
