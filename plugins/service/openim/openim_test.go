package openim

import (
	"context"
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins/service/openim/conf"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/durationpb"
)

// TestNewServiceOpenIM tests plugin creation
func TestNewServiceOpenIM(t *testing.T) {
	plugin := NewServiceOpenIM()
	assert.NotNil(t, plugin)
	assert.Equal(t, "openim.service", plugin.Name())
	assert.Equal(t, pluginVersion, plugin.Version())
	assert.Equal(t, pluginDescription, plugin.Description())
	assert.NotNil(t, plugin.messageHandlers)
	assert.NotNil(t, plugin.eventHandlers)
}

// TestServiceOpenIM_setDefaultConfig tests default configuration setting
func TestServiceOpenIM_setDefaultConfig(t *testing.T) {
	plugin := NewServiceOpenIM()
	plugin.conf = &conf.OpenIM{}

	plugin.setDefaultConfig()

	assert.NotNil(t, plugin.conf.Server)
	assert.Equal(t, "localhost:10002", plugin.conf.Server.Addr)
	assert.Equal(t, "v3", plugin.conf.Server.ApiVersion)
	assert.Equal(t, "info", plugin.conf.Server.LogLevel)

	assert.NotNil(t, plugin.conf.Client)
	assert.NotNil(t, plugin.conf.Client.Timeout)
	assert.NotNil(t, plugin.conf.Client.HeartbeatInterval)

	assert.NotNil(t, plugin.conf.Security)
	assert.NotNil(t, plugin.conf.Security.JwtExpire)

	assert.NotNil(t, plugin.conf.Storage)
	assert.Equal(t, "redis", plugin.conf.Storage.Type)
	assert.Equal(t, int32(10), plugin.conf.Storage.PoolSize)
	assert.NotNil(t, plugin.conf.Storage.Timeout)
}

// TestServiceOpenIM_initializeClient tests client initialization
func TestServiceOpenIM_initializeClient(t *testing.T) {
	plugin := NewServiceOpenIM()

	// Test with nil client config
	plugin.conf = &conf.OpenIM{}
	err := plugin.initializeClient()
	assert.NoError(t, err)
	assert.Nil(t, plugin.client)

	// Test with client config
	plugin.conf.Client = &conf.Client{
		Timeout: durationpb.New(30 * time.Second),
	}
	err = plugin.initializeClient()
	assert.NoError(t, err)
	assert.NotNil(t, plugin.client)
	assert.Equal(t, plugin.conf.Client, plugin.client.config)
}

// TestServiceOpenIM_initializeServer tests server initialization
func TestServiceOpenIM_initializeServer(t *testing.T) {
	plugin := NewServiceOpenIM()

	// Test with nil server config
	plugin.conf = &conf.OpenIM{}
	err := plugin.initializeServer()
	assert.NoError(t, err)
	assert.Nil(t, plugin.server)

	// Test with server config
	plugin.conf.Server = &conf.Server{
		Addr: "localhost:10002",
	}
	err = plugin.initializeServer()
	assert.NoError(t, err)
	assert.NotNil(t, plugin.server)
	assert.Equal(t, plugin.conf.Server, plugin.server.config)
}

// TestServiceOpenIM_startServer tests server start
func TestServiceOpenIM_startServer(t *testing.T) {
	plugin := NewServiceOpenIM()
	plugin.conf = &conf.OpenIM{
		Server: &conf.Server{
			Addr: "localhost:10002",
		},
	}
	plugin.initializeServer()

	// Test start
	err := plugin.startServer()
	assert.NoError(t, err)

	plugin.server.mu.RLock()
	running := plugin.server.running
	plugin.server.mu.RUnlock()
	assert.True(t, running)

	// Test start again (should fail)
	err = plugin.startServer()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

// TestServiceOpenIM_stopServer tests server stop
func TestServiceOpenIM_stopServer(t *testing.T) {
	plugin := NewServiceOpenIM()
	plugin.conf = &conf.OpenIM{
		Server: &conf.Server{
			Addr: "localhost:10002",
		},
	}
	plugin.initializeServer()
	plugin.startServer()

	// Test stop
	err := plugin.stopServer()
	assert.NoError(t, err)

	plugin.server.mu.RLock()
	running := plugin.server.running
	plugin.server.mu.RUnlock()
	assert.False(t, running)

	// Test stop again (should be idempotent)
	err = plugin.stopServer()
	assert.NoError(t, err)
}

// TestServiceOpenIM_startClient tests client start
func TestServiceOpenIM_startClient(t *testing.T) {
	plugin := NewServiceOpenIM()
	plugin.conf = &conf.OpenIM{
		Client: &conf.Client{
			Timeout: durationpb.New(30 * time.Second),
		},
	}
	plugin.initializeClient()

	// Test start
	err := plugin.startClient()
	assert.NoError(t, err)

	plugin.client.mu.RLock()
	connected := plugin.client.connected
	plugin.client.mu.RUnlock()
	assert.True(t, connected)

	// Test start again (should fail)
	err = plugin.startClient()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already connected")
}

// TestServiceOpenIM_stopClient tests client stop
func TestServiceOpenIM_stopClient(t *testing.T) {
	plugin := NewServiceOpenIM()
	plugin.conf = &conf.OpenIM{
		Client: &conf.Client{
			Timeout: durationpb.New(30 * time.Second),
		},
	}
	plugin.initializeClient()
	plugin.startClient()

	// Test stop
	err := plugin.stopClient()
	assert.NoError(t, err)

	plugin.client.mu.RLock()
	connected := plugin.client.connected
	plugin.client.mu.RUnlock()
	assert.False(t, connected)

	// Test stop again (should be idempotent)
	err = plugin.stopClient()
	assert.NoError(t, err)
}

// TestServiceOpenIM_checkServerHealth tests server health check
func TestServiceOpenIM_checkServerHealth(t *testing.T) {
	plugin := NewServiceOpenIM()

	// Test with nil server
	err := plugin.checkServerHealth()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	// Test with server not running
	plugin.conf = &conf.OpenIM{
		Server: &conf.Server{
			Addr: "localhost:10002",
		},
	}
	plugin.initializeServer()
	err = plugin.checkServerHealth()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")

	// Test with server running
	plugin.startServer()
	err = plugin.checkServerHealth()
	assert.NoError(t, err)
}

// TestServiceOpenIM_checkClientHealth tests client health check
func TestServiceOpenIM_checkClientHealth(t *testing.T) {
	plugin := NewServiceOpenIM()

	// Test with nil client
	err := plugin.checkClientHealth()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	// Test with client not connected
	plugin.conf = &conf.OpenIM{
		Client: &conf.Client{
			Timeout: durationpb.New(30 * time.Second),
		},
	}
	plugin.initializeClient()
	err = plugin.checkClientHealth()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")

	// Test with client connected
	plugin.startClient()
	err = plugin.checkClientHealth()
	assert.NoError(t, err)
}

// TestServiceOpenIM_RegisterMessageHandler tests message handler registration
func TestServiceOpenIM_RegisterMessageHandler(t *testing.T) {
	plugin := NewServiceOpenIM()

	handler := func(ctx context.Context, msg *conf.Message) error {
		return nil
	}

	plugin.RegisterMessageHandler("text", handler)

	plugin.mu.RLock()
	registeredHandler, exists := plugin.messageHandlers["text"]
	plugin.mu.RUnlock()

	assert.True(t, exists)
	assert.NotNil(t, registeredHandler)
}

// TestServiceOpenIM_RegisterEventHandler tests event handler registration
func TestServiceOpenIM_RegisterEventHandler(t *testing.T) {
	plugin := NewServiceOpenIM()

	handler := func(ctx context.Context, event interface{}) error {
		return nil
	}

	plugin.RegisterEventHandler("user_online", handler)

	plugin.mu.RLock()
	registeredHandler, exists := plugin.eventHandlers["user_online"]
	plugin.mu.RUnlock()

	assert.True(t, exists)
	assert.NotNil(t, registeredHandler)
}

// TestServiceOpenIM_SendMessage tests sending message
func TestServiceOpenIM_SendMessage(t *testing.T) {
	plugin := NewServiceOpenIM()

	// Test with no handler
	msg := &conf.Message{
		Type: "text",
	}
	err := plugin.SendMessage(context.Background(), msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no handler registered")

	// Test with handler
	handler := func(ctx context.Context, msg *conf.Message) error {
		return nil
	}
	plugin.RegisterMessageHandler("text", handler)

	err = plugin.SendMessage(context.Background(), msg)
	assert.NoError(t, err)
}

// TestServiceOpenIM_GetClient tests getting client
func TestServiceOpenIM_GetClient(t *testing.T) {
	plugin := NewServiceOpenIM()
	plugin.conf = &conf.OpenIM{
		Client: &conf.Client{
			Timeout: durationpb.New(30 * time.Second),
		},
	}
	plugin.initializeClient()

	client := plugin.GetClient()
	assert.NotNil(t, client)
	assert.Equal(t, plugin.client, client)
}

// TestServiceOpenIM_GetServer tests getting server
func TestServiceOpenIM_GetServer(t *testing.T) {
	plugin := NewServiceOpenIM()
	plugin.conf = &conf.OpenIM{
		Server: &conf.Server{
			Addr: "localhost:10002",
		},
	}
	plugin.initializeServer()

	server := plugin.GetServer()
	assert.NotNil(t, server)
	assert.Equal(t, plugin.server, server)
}

// TestServiceOpenIM_GetConfig tests getting configuration
func TestServiceOpenIM_GetConfig(t *testing.T) {
	plugin := NewServiceOpenIM()
	plugin.conf = &conf.OpenIM{
		Server: &conf.Server{
			Addr: "localhost:10002",
		},
	}

	config := plugin.GetConfig()
	assert.NotNil(t, config)
	assert.Equal(t, plugin.conf, config)
}

// TestServiceOpenIM_Configure tests configuration update
func TestServiceOpenIM_Configure(t *testing.T) {
	plugin := NewServiceOpenIM()
	plugin.conf = &conf.OpenIM{
		Server: &conf.Server{
			Addr: "localhost:10002",
		},
	}

	// Test nil configuration
	err := plugin.Configure(nil)
	assert.NoError(t, err)

	// Test valid configuration
	newConfig := &conf.OpenIM{
		Server: &conf.Server{
			Addr: "localhost:10003",
		},
	}
	err = plugin.Configure(newConfig)
	assert.NoError(t, err)
	assert.Equal(t, "localhost:10003", plugin.conf.Server.Addr)
}

// TestServiceOpenIM_CleanupTasks tests cleanup
func TestServiceOpenIM_CleanupTasks(t *testing.T) {
	plugin := NewServiceOpenIM()
	plugin.conf = &conf.OpenIM{
		Server: &conf.Server{
			Addr: "localhost:10002",
		},
		Client: &conf.Client{
			Timeout: durationpb.New(30 * time.Second),
		},
	}
	plugin.initializeServer()
	plugin.initializeClient()
	plugin.startServer()
	plugin.startClient()

	// Create context
	plugin.ctx, plugin.cancel = context.WithCancel(context.Background())

	// Test cleanup
	err := plugin.CleanupTasks()
	assert.NoError(t, err)

	// Verify server stopped
	plugin.server.mu.RLock()
	running := plugin.server.running
	plugin.server.mu.RUnlock()
	assert.False(t, running)

	// Verify client disconnected
	plugin.client.mu.RLock()
	connected := plugin.client.connected
	plugin.client.mu.RUnlock()
	assert.False(t, connected)
}

// TestServiceOpenIM_CheckHealth tests health check
func TestServiceOpenIM_CheckHealth(t *testing.T) {
	plugin := NewServiceOpenIM()

	// Test with no server or client
	err := plugin.CheckHealth()
	assert.NoError(t, err) // Should pass if nothing is configured

	// Test with server running
	plugin.conf = &conf.OpenIM{
		Server: &conf.Server{
			Addr: "localhost:10002",
		},
	}
	plugin.initializeServer()
	plugin.startServer()
	err = plugin.CheckHealth()
	assert.NoError(t, err)

	// Test with server not running
	plugin.stopServer()
	err = plugin.CheckHealth()
	assert.Error(t, err)
}

// TestServiceOpenIM_registerDefaultHandlers tests default handler registration
func TestServiceOpenIM_registerDefaultHandlers(t *testing.T) {
	plugin := NewServiceOpenIM()
	plugin.registerDefaultHandlers()

	plugin.mu.RLock()
	textHandler, textExists := plugin.messageHandlers["text"]
	onlineHandler, onlineExists := plugin.eventHandlers["user_online"]
	offlineHandler, offlineExists := plugin.eventHandlers["user_offline"]
	plugin.mu.RUnlock()

	assert.True(t, textExists)
	assert.NotNil(t, textHandler)
	assert.True(t, onlineExists)
	assert.NotNil(t, onlineHandler)
	assert.True(t, offlineExists)
	assert.NotNil(t, offlineHandler)
}

