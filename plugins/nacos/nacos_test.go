package nacos

import (
	"testing"

	"github.com/go-lynx/lynx/plugins/nacos/conf"
	"github.com/stretchr/testify/assert"
)

// TestNewNacosControlPlane tests plugin creation
func TestNewNacosControlPlane(t *testing.T) {
	plugin := NewNacosControlPlane()
	assert.NotNil(t, plugin)
	assert.Equal(t, pluginName, plugin.Name())
	assert.Equal(t, pluginVersion, plugin.Version())
	assert.Equal(t, pluginDescription, plugin.Description())
	assert.NotNil(t, plugin.configWatchers)
	assert.NotNil(t, plugin.serviceCache)
	assert.NotNil(t, plugin.configCache)
}

// TestPlugNacos_setDefaultConfig tests default configuration setting
func TestPlugNacos_setDefaultConfig(t *testing.T) {
	plugin := NewNacosControlPlane()
	plugin.conf = &conf.Nacos{}

	plugin.setDefaultConfig()

	assert.Equal(t, conf.DefaultNamespace, plugin.conf.Namespace)
	assert.Equal(t, conf.DefaultWeight, plugin.conf.Weight)
	assert.Equal(t, conf.DefaultTimeout, plugin.conf.Timeout)
	assert.Equal(t, conf.DefaultNotifyTimeout, plugin.conf.NotifyTimeout)
	assert.Equal(t, conf.DefaultLogLevel, plugin.conf.LogLevel)
	assert.Equal(t, conf.DefaultLogDir, plugin.conf.LogDir)
	assert.Equal(t, conf.DefaultCacheDir, plugin.conf.CacheDir)
	assert.Equal(t, conf.DefaultContextPath, plugin.conf.ContextPath)
}

// TestPlugNacos_validateConfig tests configuration validation
func TestPlugNacos_validateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *conf.Nacos
		wantErr bool
	}{
		{
			name: "valid config with server addresses",
			config: &conf.Nacos{
				ServerAddresses: "127.0.0.1:8848",
			},
			wantErr: false,
		},
		{
			name: "valid config with endpoint",
			config: &conf.Nacos{
				Endpoint: "http://nacos.example.com",
			},
			wantErr: false,
		},
		{
			name: "invalid config - no server addresses or endpoint",
			config: &conf.Nacos{
				ServerAddresses: "",
				Endpoint:        "",
			},
			wantErr: true,
		},
		{
			name: "valid config with service config",
			config: &conf.Nacos{
				ServerAddresses: "127.0.0.1:8848",
				EnableRegister:  true,
				ServiceConfig: &conf.ServiceConfig{
					ServiceName: "test-service",
					Group:       "DEFAULT_GROUP",
					Cluster:     "DEFAULT",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid config - http health check without URL",
			config: &conf.Nacos{
				ServerAddresses: "127.0.0.1:8848",
				EnableRegister:  true,
				ServiceConfig: &conf.ServiceConfig{
					ServiceName:     "test-service",
					HealthCheck:     true,
					HealthCheckType: "http",
					HealthCheckUrl:  "",
				},
			},
			wantErr: true,
		},
		{
			name: "valid config with additional configs",
			config: &conf.Nacos{
				ServerAddresses: "127.0.0.1:8848",
				AdditionalConfigs: []*conf.AdditionalConfig{
					{
						DataId: "test-config",
						Group:  "DEFAULT_GROUP",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid config - additional config without dataId",
			config: &conf.Nacos{
				ServerAddresses: "127.0.0.1:8848",
				AdditionalConfigs: []*conf.AdditionalConfig{
					{
						DataId: "",
						Group:  "DEFAULT_GROUP",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := NewNacosControlPlane()
			plugin.conf = tt.config
			plugin.setDefaultConfig()

			err := plugin.validateConfig()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestPlugNacos_validateServiceConfig tests service configuration validation
func TestPlugNacos_validateServiceConfig(t *testing.T) {
	plugin := NewNacosControlPlane()

	tests := []struct {
		name    string
		config  *conf.ServiceConfig
		wantErr bool
	}{
		{
			name: "valid service config",
			config: &conf.ServiceConfig{
				ServiceName: "test-service",
				Group:       "DEFAULT_GROUP",
				Cluster:     "DEFAULT",
			},
			wantErr: false,
		},
		{
			name: "valid service config with health check",
			config: &conf.ServiceConfig{
				ServiceName:     "test-service",
				HealthCheck:     true,
				HealthCheckType: "tcp",
			},
			wantErr: false,
		},
		{
			name: "invalid service config - http health check without URL",
			config: &conf.ServiceConfig{
				ServiceName:     "test-service",
				HealthCheck:     true,
				HealthCheckType: "http",
				HealthCheckUrl:  "",
			},
			wantErr: true,
		},
		{
			name: "valid service config - http health check with URL",
			config: &conf.ServiceConfig{
				ServiceName:     "test-service",
				HealthCheck:     true,
				HealthCheckType: "http",
				HealthCheckUrl:  "http://localhost:8080/health",
			},
			wantErr: false,
		},
		{
			name:    "nil service config",
			config:  nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := plugin.validateServiceConfig(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestNormalizeServerAddresses tests server address normalization
func TestNormalizeServerAddresses(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{"127.0.0.1:8848"},
		},
		{
			name:     "single address",
			input:    "127.0.0.1:8848",
			expected: []string{"127.0.0.1:8848"},
		},
		{
			name:     "multiple addresses",
			input:    "127.0.0.1:8848,127.0.0.1:8849",
			expected: []string{"127.0.0.1:8848", "127.0.0.1:8849"},
		},
		{
			name:     "addresses with spaces",
			input:    "127.0.0.1:8848, 127.0.0.1:8849",
			expected: []string{"127.0.0.1:8848", "127.0.0.1:8849"},
		},
		{
			name:     "address with http prefix",
			input:    "http://127.0.0.1:8848",
			expected: []string{"127.0.0.1:8848"},
		},
		{
			name:     "address with https prefix",
			input:    "https://127.0.0.1:8848",
			expected: []string{"127.0.0.1:8848"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeServerAddresses(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestPlugNacos_checkInitialized tests initialization check
func TestPlugNacos_checkInitialized(t *testing.T) {
	plugin := NewNacosControlPlane()

	// Test not initialized
	err := plugin.checkInitialized()
	assert.Error(t, err)
	assert.Equal(t, ErrNotInitialized, err)

	// Test initialized
	atomic.StoreInt32(&plugin.initialized, 1)
	err = plugin.checkInitialized()
	assert.NoError(t, err)

	// Test destroyed
	atomic.StoreInt32(&plugin.destroyed, 1)
	err = plugin.checkInitialized()
	assert.Error(t, err)
}

// TestGetFileExtension tests file extension extraction
func TestGetFileExtension(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{
			name:     "yaml file",
			filename: "config.yaml",
			expected: "yaml",
		},
		{
			name:     "yml file",
			filename: "config.yml",
			expected: "yaml",
		},
		{
			name:     "json file",
			filename: "config.json",
			expected: "json",
		},
		{
			name:     "properties file",
			filename: "config.properties",
			expected: "properties",
		},
		{
			name:     "props file",
			filename: "config.props",
			expected: "properties",
		},
		{
			name:     "xml file",
			filename: "config.xml",
			expected: "xml",
		},
		{
			name:     "no extension",
			filename: "config",
			expected: "",
		},
		{
			name:     "empty string",
			filename: "",
			expected: "",
		},
		{
			name:     "multiple dots",
			filename: "config.test.yaml",
			expected: "yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getFileExtension(tt.filename)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestPlugNacos_Configure tests configuration update
func TestPlugNacos_Configure(t *testing.T) {
	plugin := NewNacosControlPlane()
	plugin.conf = &conf.Nacos{
		ServerAddresses: "127.0.0.1:8848",
	}

	// Test nil configuration
	err := plugin.Configure(nil)
	assert.Error(t, err)

	// Test invalid configuration type
	err = plugin.Configure("invalid")
	assert.Error(t, err)

	// Test valid configuration
	newConfig := &conf.Nacos{
		ServerAddresses: "127.0.0.1:8849",
		Namespace:      "test-namespace",
	}
	err = plugin.Configure(newConfig)
	assert.NoError(t, err)
	assert.Equal(t, "127.0.0.1:8849", plugin.conf.ServerAddresses)
	assert.Equal(t, "test-namespace", plugin.conf.Namespace)

	// Test invalid configuration (should rollback)
	invalidConfig := &conf.Nacos{
		ServerAddresses: "",
		Endpoint:        "",
	}
	oldConfig := plugin.conf
	err = plugin.Configure(invalidConfig)
	assert.Error(t, err)
	// Should rollback to old config
	assert.Equal(t, oldConfig.ServerAddresses, plugin.conf.ServerAddresses)
}

// TestPlugNacos_CleanupTasks tests cleanup
func TestPlugNacos_CleanupTasks(t *testing.T) {
	plugin := NewNacosControlPlane()
	atomic.StoreInt32(&plugin.initialized, 1)

	// Add a watcher
	watcher := &ConfigWatcher{}
	plugin.watcherMutex.Lock()
	plugin.configWatchers["test:group"] = watcher
	plugin.watcherMutex.Unlock()

	// Test cleanup
	err := plugin.CleanupTasks()
	assert.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&plugin.destroyed))
	assert.Empty(t, plugin.configWatchers)
}

// TestPlugNacos_StartupTasks tests startup
func TestPlugNacos_StartupTasks(t *testing.T) {
	plugin := NewNacosControlPlane()
	atomic.StoreInt32(&plugin.initialized, 1)

	err := plugin.StartupTasks()
	assert.NoError(t, err)
}

// TestErrorWrapping tests error wrapping functions
func TestErrorWrapping(t *testing.T) {
	baseErr := assert.AnError

	// Test WrapInitError
	wrapped := WrapInitError(baseErr, "test message")
	assert.Error(t, wrapped)
	assert.Contains(t, wrapped.Error(), "test message")

	// Test WrapOperationError
	wrapped = WrapOperationError(baseErr, "test operation")
	assert.Error(t, wrapped)
	assert.Contains(t, wrapped.Error(), "test operation")
}

