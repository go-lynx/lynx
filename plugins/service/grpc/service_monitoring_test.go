package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/service/grpc/conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestRecordHealthCheckMetrics(t *testing.T) {
	plugin := NewGrpcService()
	plugin.conf = &conf.Service{Addr: ":9090"}

	// Test recording health check metrics
	plugin.recordHealthCheckMetricsInternal(true)
	plugin.recordHealthCheckMetricsInternal(false)

	// Since metrics are global, mainly test that methods don't panic
	assert.NotNil(t, plugin)
}

func TestRecordRequestMetrics(t *testing.T) {
	plugin := NewGrpcService()

	// Test recording request metrics
	plugin.recordRequestMetrics("test.Method", 100*time.Millisecond, "success")
	plugin.recordRequestMetrics("test.Method", 200*time.Millisecond, "error")

	// Since metrics are global, mainly test that methods don't panic
	assert.NotNil(t, plugin)
}

func TestUpdateConnectionMetrics(t *testing.T) {
	plugin := NewGrpcService()

	// Test updating connection metrics
	plugin.updateConnectionMetrics(10)
	plugin.updateConnectionMetrics(0)

	// Since metrics are global, mainly test that methods don't panic
	assert.NotNil(t, plugin)
}

func TestRecordServerStartTime(t *testing.T) {
	plugin := NewGrpcService()

	// Test recording server start time
	plugin.recordServerStartTime()

	// Since metrics are global, mainly test that methods don't panic
	assert.NotNil(t, plugin)
}

func TestRecordServerError(t *testing.T) {
	plugin := NewGrpcService()

	// Test recording server errors
	plugin.recordServerError("test_error")
	plugin.recordServerError("panic_recovery")

	// Since metrics are global, mainly test that methods don't panic
	assert.NotNil(t, plugin)
}

func TestGetMetricsHandler(t *testing.T) {
	plugin := NewGrpcService()

	// Get metrics handler
	handler := plugin.getMetricsHandler()
	assert.NotNil(t, handler)

	// Test handler function signature
	// Mainly test that the method returns a non-nil function
	// Actual middleware testing needs to be done in integration tests
}

// Mock gRPC handler
func mockHandler(ctx context.Context, req interface{}) (interface{}, error) {
	return "response", nil
}

func TestMetricsHandlerIntegration(t *testing.T) {
	plugin := NewGrpcService()

	// Test metrics handler integration with proper mock setup
	mockRT := &mockRuntimeMonitoring{}
	err := plugin.InitializeResources(mockRT)
	require.NoError(t, err)

	// Test metrics recording methods (since Service doesn't have metrics field)
	plugin.recordRequestMetrics("TestMethod", 100*time.Millisecond, "success")
	plugin.recordServerError("test error")
	plugin.recordHealthCheckMetricsInternal(true)

	// Verify plugin is properly initialized
	assert.NotNil(t, plugin)
	assert.NotNil(t, plugin.conf)
}

// mockRuntimeMonitoring implements plugins.Runtime interface for monitoring tests
type mockRuntimeMonitoring struct{}

func (m *mockRuntimeMonitoring) GetConfig() config.Config {
	return &mockConfigMonitoring{}
}

func (m *mockRuntimeMonitoring) AddListener(listener plugins.EventListener, filter *plugins.EventFilter) {
}
func (m *mockRuntimeMonitoring) AddPluginListener(pluginName string, listener plugins.EventListener, filter *plugins.EventFilter) {
}
func (m *mockRuntimeMonitoring) CleanupResources(pluginName string) error { return nil }
func (m *mockRuntimeMonitoring) EmitEvent(event plugins.PluginEvent)      {}
func (m *mockRuntimeMonitoring) EmitPluginEvent(pluginName string, eventType string, data map[string]any) {
}
func (m *mockRuntimeMonitoring) GetCurrentPluginContext() string { return "" }
func (m *mockRuntimeMonitoring) GetEventHistory(filter plugins.EventFilter) []plugins.PluginEvent {
	return nil
}
func (m *mockRuntimeMonitoring) GetEventStats() map[string]any { return nil }
func (m *mockRuntimeMonitoring) GetLogger() log.Logger         { return log.DefaultLogger }
func (m *mockRuntimeMonitoring) GetPluginEventHistory(pluginName string, filter plugins.EventFilter) []plugins.PluginEvent {
	return nil
}
func (m *mockRuntimeMonitoring) GetResourceStats() map[string]any            { return nil }
func (m *mockRuntimeMonitoring) GetSharedResource(name string) (any, error)  { return nil, nil }
func (m *mockRuntimeMonitoring) GetPrivateResource(name string) (any, error) { return nil, nil }
func (m *mockRuntimeMonitoring) GetResource(name string) (any, error)        { return nil, nil }
func (m *mockRuntimeMonitoring) GetResourceInfo(name string) (*plugins.ResourceInfo, error) {
	return nil, nil
}
func (m *mockRuntimeMonitoring) ListResources() []*plugins.ResourceInfo                  { return nil }
func (m *mockRuntimeMonitoring) RegisterPrivateResource(name string, resource any) error { return nil }
func (m *mockRuntimeMonitoring) RegisterResource(name string, resource any) error        { return nil }
func (m *mockRuntimeMonitoring) RegisterSharedResource(name string, resource any) error  { return nil }
func (m *mockRuntimeMonitoring) RemoveListener(listener plugins.EventListener)           {}
func (m *mockRuntimeMonitoring) RemovePluginListener(pluginName string, listener plugins.EventListener) {
}
func (m *mockRuntimeMonitoring) SetConfig(conf config.Config)                        {}
func (m *mockRuntimeMonitoring) SetEventDispatchMode(mode string) error              { return nil }
func (m *mockRuntimeMonitoring) SetEventTimeout(timeout time.Duration)               {}
func (m *mockRuntimeMonitoring) SetEventWorkerPoolSize(size int)                     {}
func (m *mockRuntimeMonitoring) UnregisterPrivateResource(name string) error         { return nil }
func (m *mockRuntimeMonitoring) UnregisterResource(name string) error                { return nil }
func (m *mockRuntimeMonitoring) UnregisterSharedResource(name string) error          { return nil }
func (m *mockRuntimeMonitoring) WithPluginContext(pluginName string) plugins.Runtime { return m }

type mockConfigMonitoring struct{}

func (m *mockConfigMonitoring) Value(key string) config.Value {
	return &mockValueMonitoring{}
}

func (m *mockConfigMonitoring) Load() error                               { return nil }
func (m *mockConfigMonitoring) Watch(key string, o config.Observer) error { return nil }
func (m *mockConfigMonitoring) Close() error                              { return nil }
func (m *mockConfigMonitoring) Scan(dest interface{}) error               { return nil }

type mockValueMonitoring struct{}

func (m *mockValueMonitoring) Scan(dest interface{}) error {
	if service, ok := dest.(*conf.Service); ok {
		service.Addr = ":9090"
		service.Network = "tcp"
		service.Timeout = &durationpb.Duration{Seconds: 30}
	}
	return nil
}

func (m *mockValueMonitoring) Bool() (bool, error)                   { return false, nil }
func (m *mockValueMonitoring) Int() (int64, error)                   { return 0, nil }
func (m *mockValueMonitoring) Float() (float64, error)               { return 0, nil }
func (m *mockValueMonitoring) String() (string, error)               { return "", nil }
func (m *mockValueMonitoring) Duration() (time.Duration, error)      { return 0, nil }
func (m *mockValueMonitoring) Slice() ([]config.Value, error)        { return nil, nil }
func (m *mockValueMonitoring) Map() (map[string]config.Value, error) { return nil, nil }
func (m *mockValueMonitoring) Load() any                             { return nil }
func (m *mockValueMonitoring) Store(any)                             {}
