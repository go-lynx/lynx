package openim

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/service/openim/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	// pluginVersion indicates the current version of the plugin
	pluginVersion = "v1.0.0"

	// pluginDescription provides a brief description of the plugin's functionality
	pluginDescription = "OpenIM instant messaging service plugin for lynx framework"
)

// ServiceOpenIM represents the OpenIM service plugin implementation.
// It embeds the BasePlugin for common plugin functionality and maintains
// the OpenIM service instance along with its configuration.
type ServiceOpenIM struct {
	// Embed Lynx framework's base plugin, inheriting common plugin functionality
	*plugins.BasePlugin
	// OpenIM service configuration
	conf *conf.OpenIM
	// OpenIM client instance
	client *OpenIMClient
	// OpenIM server instance
	server *OpenIMServer
	// Message handlers
	messageHandlers map[string]MessageHandler
	// Event handlers
	eventHandlers map[string]EventHandler
	// Mutex for thread safety
	mu sync.RWMutex
	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// MessageHandler defines the interface for handling different message types
type MessageHandler func(ctx context.Context, msg *conf.Message) error

// EventHandler defines the interface for handling different events
type EventHandler func(ctx context.Context, event interface{}) error

// OpenIMClient represents an OpenIM client instance
type OpenIMClient struct {
	config *conf.Client
	// Add more client fields as needed
}

// OpenIMServer represents an OpenIM server instance
type OpenIMServer struct {
	config *conf.Server
	// Add more server fields as needed
}

// NewServiceOpenIM creates and initializes a new instance of the OpenIM service plugin.
// It sets up the base plugin with the appropriate metadata and returns a pointer
// to the ServiceOpenIM structure.
func NewServiceOpenIM() *ServiceOpenIM {
	return &ServiceOpenIM{
		BasePlugin: plugins.NewBasePlugin(
			// Generate unique plugin ID
			plugins.GeneratePluginID("", "openim.service", pluginVersion),
			// Plugin name
			"openim.service",
			// Plugin description
			pluginDescription,
			// Plugin version
			pluginVersion,
			// Configuration prefix
			"lynx.openim",
			// Weight
			15,
		),
		messageHandlers: make(map[string]MessageHandler),
		eventHandlers:   make(map[string]EventHandler),
	}
}

// InitializeResources implements the plugin initialization interface.
// It loads and validates the OpenIM service configuration from the runtime environment.
// If no configuration is provided, it sets up default values for the service.
func (o *ServiceOpenIM) InitializeResources(rt plugins.Runtime) error {
	// Initialize an empty configuration structure
	o.conf = &conf.OpenIM{}

	// Scan and load OpenIM configuration from runtime configuration
	err := rt.GetConfig().Value("lynx.openim").Scan(o.conf)
	if err != nil {
		return err
	}

	// Set default configuration if not provided
	o.setDefaultConfig()

	// Initialize client and server
	if err := o.initializeClient(); err != nil {
		return err
	}

	if err := o.initializeServer(); err != nil {
		return err
	}

	// Register default message handlers
	o.registerDefaultHandlers()

	return nil
}

// StartupTasks implements the plugin startup interface.
// It starts the OpenIM service and begins listening for messages and events.
func (o *ServiceOpenIM) StartupTasks() error {
	// Log OpenIM service startup
	log.Infof("starting OpenIM service")

	// Create context for cancellation
	o.ctx, o.cancel = context.WithCancel(context.Background())

	// Start the server if configured
	if o.server != nil {
		if err := o.startServer(); err != nil {
			return err
		}
	}

	// Start the client if configured
	if o.client != nil {
		if err := o.startClient(); err != nil {
			return err
		}
	}

	// Log successful OpenIM service startup
	log.Infof("OpenIM service successfully started")
	return nil
}

// CleanupTasks implements the plugin cleanup interface.
// It gracefully stops the OpenIM service and performs necessary cleanup operations.
func (o *ServiceOpenIM) CleanupTasks() error {
	// Cancel context to stop all goroutines
	if o.cancel != nil {
		o.cancel()
	}

	// Stop server if running
	if o.server != nil {
		if err := o.stopServer(); err != nil {
			return plugins.NewPluginError(o.ID(), "Stop", "Failed to stop OpenIM server", err)
		}
	}

	// Stop client if running
	if o.client != nil {
		if err := o.stopClient(); err != nil {
			return plugins.NewPluginError(o.ID(), "Stop", "Failed to stop OpenIM client", err)
		}
	}

	return nil
}

// Configure allows runtime configuration updates for the OpenIM service.
// It accepts an interface{} parameter that should contain the new configuration
// and updates the service settings accordingly.
func (o *ServiceOpenIM) Configure(c any) error {
	if c == nil {
		return nil
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	// Convert the incoming configuration to *conf.OpenIM type and update service configuration
	if newConf, ok := c.(*conf.OpenIM); ok {
		o.conf = newConf
		// Reinitialize client and server with new configuration
		if err := o.initializeClient(); err != nil {
			return err
		}
		if err := o.initializeServer(); err != nil {
			return err
		}
	}

	return nil
}

// CheckHealth implements the health check interface for the OpenIM service.
// It performs necessary health checks and returns the current status.
func (o *ServiceOpenIM) CheckHealth() error {
	// Check server health
	if o.server != nil {
		if err := o.checkServerHealth(); err != nil {
			return err
		}
	}

	// Check client health
	if o.client != nil {
		if err := o.checkClientHealth(); err != nil {
			return err
		}
	}

	return nil
}

// setDefaultConfig sets default configuration values for the OpenIM service
func (o *ServiceOpenIM) setDefaultConfig() {
	if o.conf.Server == nil {
		o.conf.Server = &conf.Server{}
	}
	if o.conf.Server.Addr == "" {
		o.conf.Server.Addr = "localhost:10002"
	}
	if o.conf.Server.ApiVersion == "" {
		o.conf.Server.ApiVersion = "v3"
	}
	if o.conf.Server.LogLevel == "" {
		o.conf.Server.LogLevel = "info"
	}

	if o.conf.Client == nil {
		o.conf.Client = &conf.Client{}
	}
	if o.conf.Client.Timeout == nil {
		o.conf.Client.Timeout = durationpb.New(30 * time.Second)
	}
	if o.conf.Client.HeartbeatInterval == nil {
		o.conf.Client.HeartbeatInterval = durationpb.New(30 * time.Second)
	}

	if o.conf.Security == nil {
		o.conf.Security = &conf.Security{}
	}
	if o.conf.Security.JwtExpire == nil {
		o.conf.Security.JwtExpire = durationpb.New(24 * time.Hour)
	}

	if o.conf.Storage == nil {
		o.conf.Storage = &conf.Storage{}
	}
	if o.conf.Storage.Type == "" {
		o.conf.Storage.Type = "redis"
	}
	if o.conf.Storage.PoolSize == 0 {
		o.conf.Storage.PoolSize = 10
	}
	if o.conf.Storage.Timeout == nil {
		o.conf.Storage.Timeout = durationpb.New(5 * time.Second)
	}
}

// initializeClient initializes the OpenIM client
func (o *ServiceOpenIM) initializeClient() error {
	if o.conf.Client == nil {
		return nil
	}

	o.client = &OpenIMClient{
		config: o.conf.Client,
	}

	return nil
}

// initializeServer initializes the OpenIM server
func (o *ServiceOpenIM) initializeServer() error {
	if o.conf.Server == nil {
		return nil
	}

	o.server = &OpenIMServer{
		config: o.conf.Server,
	}

	return nil
}

// startServer starts the OpenIM server
func (o *ServiceOpenIM) startServer() error {
	if o.server == nil {
		return fmt.Errorf("server not initialized")
	}

	// Start server logic here
	log.Infof("OpenIM server started on %s", o.server.config.Addr)
	return nil
}

// stopServer stops the OpenIM server
func (o *ServiceOpenIM) stopServer() error {
	if o.server == nil {
		return nil
	}

	// Stop server logic here
	log.Infof("OpenIM server stopped")
	return nil
}

// startClient starts the OpenIM client
func (o *ServiceOpenIM) startClient() error {
	if o.client == nil {
		return fmt.Errorf("client not initialized")
	}

	// Start client logic here
	log.Infof("OpenIM client started")
	return nil
}

// stopClient stops the OpenIM client
func (o *ServiceOpenIM) stopClient() error {
	if o.client == nil {
		return nil
	}

	// Stop client logic here
	log.Infof("OpenIM client stopped")
	return nil
}

// checkServerHealth checks the health of the OpenIM server
func (o *ServiceOpenIM) checkServerHealth() error {
	// Implement server health check logic
	return nil
}

// checkClientHealth checks the health of the OpenIM client
func (o *ServiceOpenIM) checkClientHealth() error {
	// Implement client health check logic
	return nil
}

// registerDefaultHandlers registers default message and event handlers
func (o *ServiceOpenIM) registerDefaultHandlers() {
	// Register default text message handler
	o.RegisterMessageHandler("text", o.handleTextMessage)

	// Register default event handlers
	o.RegisterEventHandler("user_online", o.handleUserOnline)
	o.RegisterEventHandler("user_offline", o.handleUserOffline)
}

// handleTextMessage handles text messages
func (o *ServiceOpenIM) handleTextMessage(ctx context.Context, msg *conf.Message) error {
	log.Infof("Handling text message from %s to %s: %s", msg.Sender, msg.Receiver, msg.Content)
	return nil
}

// handleUserOnline handles user online events
func (o *ServiceOpenIM) handleUserOnline(ctx context.Context, event interface{}) error {
	log.Infof("User online event: %v", event)
	return nil
}

// handleUserOffline handles user offline events
func (o *ServiceOpenIM) handleUserOffline(ctx context.Context, event interface{}) error {
	log.Infof("User offline event: %v", event)
	return nil
}

// RegisterMessageHandler registers a message handler for a specific message type
func (o *ServiceOpenIM) RegisterMessageHandler(msgType string, handler MessageHandler) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.messageHandlers[msgType] = handler
}

// RegisterEventHandler registers an event handler for a specific event type
func (o *ServiceOpenIM) RegisterEventHandler(eventType string, handler EventHandler) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.eventHandlers[eventType] = handler
}

// SendMessage sends a message using the OpenIM service
func (o *ServiceOpenIM) SendMessage(ctx context.Context, msg *conf.Message) error {
	o.mu.RLock()
	handler, exists := o.messageHandlers[msg.Type]
	o.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no handler registered for message type: %s", msg.Type)
	}

	return handler(ctx, msg)
}

// GetClient returns the OpenIM client instance
func (o *ServiceOpenIM) GetClient() *OpenIMClient {
	return o.client
}

// GetServer returns the OpenIM server instance
func (o *ServiceOpenIM) GetServer() *OpenIMServer {
	return o.server
}

// GetConfig returns the OpenIM configuration
func (o *ServiceOpenIM) GetConfig() *conf.OpenIM {
	return o.conf
}
