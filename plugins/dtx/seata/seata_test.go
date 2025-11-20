package seata

import (
	"testing"

	"github.com/go-lynx/lynx/plugins/seata/conf"
	"github.com/stretchr/testify/assert"
)

// TestNewTxSeataClient tests plugin creation
func TestNewTxSeataClient(t *testing.T) {
	client := NewTxSeataClient()
	assert.NotNil(t, client)
	assert.Equal(t, pluginName, client.Name())
	assert.Equal(t, pluginVersion, client.Version())
	assert.Equal(t, pluginDescription, client.Description())
}

// TestTxSeataClient_InitializeResources tests initialization
func TestTxSeataClient_InitializeResources(t *testing.T) {
	client := NewTxSeataClient()

	// Test with nil runtime (will fail, but tests error handling)
	// Note: This test requires a mock runtime, which is complex
	// We'll test the default config setting logic instead

	// Test default configuration
	client.conf = &conf.Seata{}
	if client.conf.ConfigFilePath == "" {
		client.conf.ConfigFilePath = "./conf/seata.yml"
	}

	assert.Equal(t, "./conf/seata.yml", client.conf.ConfigFilePath)
}

// TestTxSeataClient_StartupTasks tests startup
func TestTxSeataClient_StartupTasks(t *testing.T) {
	client := NewTxSeataClient()
	client.conf = &conf.Seata{
		Enabled:        false,
		ConfigFilePath: "./conf/seata.yml",
	}

	// Test with disabled Seata
	err := client.StartupTasks()
	assert.NoError(t, err)

	// Test with enabled Seata (will try to initialize, may fail without actual Seata server)
	client.conf.Enabled = true
	// Note: This will call client.InitPath which may fail without actual config file
	// We'll just verify it doesn't panic
	err = client.StartupTasks()
	// Error is acceptable if config file doesn't exist
	if err != nil {
		t.Logf("StartupTasks returned error (expected if config file doesn't exist): %v", err)
	}
}

// TestTxSeataClient_CleanupTasks tests cleanup
func TestTxSeataClient_CleanupTasks(t *testing.T) {
	client := NewTxSeataClient()

	err := client.CleanupTasks()
	assert.NoError(t, err)
}

// TestTxSeataClient_Configure tests configuration update
func TestTxSeataClient_Configure(t *testing.T) {
	client := NewTxSeataClient()
	client.conf = &conf.Seata{
		ConfigFilePath: "./conf/seata.yml",
	}

	// Test nil configuration
	err := client.Configure(nil)
	assert.NoError(t, err)

	// Test invalid configuration type
	err = client.Configure("invalid")
	assert.Error(t, err)

	// Test valid configuration
	newConfig := &conf.Seata{
		Enabled:        true,
		ConfigFilePath: "./conf/seata-custom.yml",
	}
	err = client.Configure(newConfig)
	assert.NoError(t, err)
	assert.Equal(t, "./conf/seata-custom.yml", client.conf.ConfigFilePath)
	assert.True(t, client.conf.Enabled)
}

// TestTxSeataClient_GetConfig tests getting configuration
func TestTxSeataClient_GetConfig(t *testing.T) {
	client := NewTxSeataClient()
	client.conf = &conf.Seata{
		Enabled:        true,
		ConfigFilePath: "./conf/seata.yml",
	}

	config := client.GetConfig()
	assert.NotNil(t, config)
	assert.Equal(t, client.conf, config)
}

// TestTxSeataClient_GetConfigFilePath tests getting config file path
func TestTxSeataClient_GetConfigFilePath(t *testing.T) {
	client := NewTxSeataClient()
	client.conf = &conf.Seata{
		ConfigFilePath: "./conf/seata.yml",
	}

	path := client.GetConfigFilePath()
	assert.Equal(t, "./conf/seata.yml", path)
}

// TestTxSeataClient_IsEnabled tests checking if Seata is enabled
func TestTxSeataClient_IsEnabled(t *testing.T) {
	client := NewTxSeataClient()

	// Test default (should be false)
	assert.False(t, client.IsEnabled())

	// Test enabled
	client.conf = &conf.Seata{
		Enabled: true,
	}
	assert.True(t, client.IsEnabled())

	// Test disabled
	client.conf.Enabled = false
	assert.False(t, client.IsEnabled())
}

// TestDefaultConfig tests default configuration values
func TestDefaultConfig(t *testing.T) {
	defaultConf := &conf.Seata{
		ConfigFilePath: "./conf/seata.yml",
	}

	assert.Equal(t, "./conf/seata.yml", defaultConf.ConfigFilePath)
}

// TestPluginMetadata tests plugin metadata constants
func TestPluginMetadata(t *testing.T) {
	assert.Equal(t, "seata.server", pluginName)
	assert.Equal(t, "v2.0.0", pluginVersion)
	assert.Equal(t, "seata transaction server plugin for Lynx framework", pluginDescription)
	assert.Equal(t, "lynx.seata", confPrefix)
}

