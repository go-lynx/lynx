package apollo

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/apollo/conf"
)

// Plugin metadata
// Plugin metadata defining basic plugin information
const (
	// pluginName is the unique identifier for the Apollo configuration center plugin, used to identify this plugin in the plugin system.
	pluginName = "apollo.config.center"

	// pluginVersion represents the current version of the Apollo configuration center plugin.
	pluginVersion = "v1.0.0"

	// pluginDescription briefly describes the functionality of the Apollo configuration center plugin.
	pluginDescription = "apollo configuration center plugin for lynx framework"

	// confPrefix is the configuration prefix used when loading Apollo configuration.
	confPrefix = "lynx.apollo"
)

// PlugApollo represents an Apollo configuration center plugin instance
type PlugApollo struct {
	*plugins.BasePlugin
	conf *conf.Apollo

	// Apollo HTTP client
	client *ApolloHTTPClient

	// Enhanced components
	metrics        *Metrics
	retryManager   *RetryManager
	circuitBreaker *CircuitBreaker

	// State management - using atomic operations to improve concurrency safety
	mu                  sync.RWMutex
	initialized         int32 // Use int32 instead of bool to support atomic operations
	destroyed           int32 // Use int32 instead of bool to support atomic operations
	healthCheckCh       chan struct{}
	healthCheckCloseOnce sync.Once // Protect against multiple close operations

	// Configuration watchers
	configWatchers map[string]*ConfigWatcher
	watcherMutex   sync.RWMutex // Watcher mutex

	// Cache system
	configCache map[string]interface{} // Configuration cache
	cacheMutex  sync.RWMutex           // Cache mutex
}

// NewApolloConfigCenter creates a new Apollo configuration center.
// This function initializes the plugin's basic information and returns a pointer to PlugApollo.
func NewApolloConfigCenter() *PlugApollo {
	return &PlugApollo{
		BasePlugin: plugins.NewBasePlugin(
			// Generate unique plugin ID
			plugins.GeneratePluginID("", pluginName, pluginVersion),
			// Plugin name
			pluginName,
			// Plugin description
			pluginDescription,
			// Plugin version
			pluginVersion,
			// Configuration prefix
			confPrefix,
			// Weight
			math.MaxInt,
		),
		healthCheckCh:  make(chan struct{}),
		configWatchers: make(map[string]*ConfigWatcher),
	}
}

// InitializeResources implements custom initialization logic for the Apollo plugin.
// This function loads and validates Apollo configuration, using default configuration if none is provided.
func (p *PlugApollo) InitializeResources(rt plugins.Runtime) error {
	// Initialize an empty configuration structure
	p.conf = &conf.Apollo{}

	// Scan and load Apollo configuration from runtime configuration
	err := rt.GetConfig().Value(confPrefix).Scan(p.conf)
	if err != nil {
		return WrapInitError(err, "failed to scan apollo configuration")
	}

	// Set default configuration
	p.setDefaultConfig()

	// Validate configuration
	if err := p.validateConfig(); err != nil {
		return WrapInitError(err, "configuration validation failed")
	}

	// Initialize enhanced components
	if err := p.initComponents(); err != nil {
		return WrapInitError(err, "failed to initialize components")
	}

	return nil
}

// setDefaultConfig sets default configuration
func (p *PlugApollo) setDefaultConfig() {
	// Default cluster is 'default'
	if p.conf.Cluster == "" {
		p.conf.Cluster = conf.DefaultCluster
	}
	// Default namespace is 'application'
	if p.conf.Namespace == "" {
		p.conf.Namespace = conf.DefaultNamespace
	}
	// Default timeout is 10 seconds
	if p.conf.Timeout == nil {
		p.conf.Timeout = conf.GetDefaultTimeout()
	}
	// Default notification timeout is 30 seconds
	if p.conf.NotificationTimeout == nil {
		p.conf.NotificationTimeout = conf.GetDefaultNotificationTimeout()
	}
	// Default cache directory
	if p.conf.CacheDir == "" {
		p.conf.CacheDir = conf.DefaultCacheDir
	}
}

// validateConfig validates configuration
func (p *PlugApollo) validateConfig() error {
	if p.conf == nil {
		return NewConfigError("configuration is required")
	}

	validator := NewValidator(p.conf)
	result := validator.Validate()
	if !result.IsValid {
		return NewConfigError(result.Errors[0].Error())
	}

	return nil
}

// initComponents initializes enhanced components
func (p *PlugApollo) initComponents() error {
	// Initialize monitoring metrics
	p.metrics = NewApolloMetrics()

	// Initialize retry manager
	p.retryManager = NewRetryManager(3, time.Second)

	// Initialize circuit breaker
	p.circuitBreaker = NewCircuitBreaker(0.5)

	return nil
}

// checkInitialized unified state checking method ensuring thread safety
func (p *PlugApollo) checkInitialized() error {
	if atomic.LoadInt32(&p.initialized) == 0 {
		return NewInitError("Apollo plugin not initialized")
	}
	if atomic.LoadInt32(&p.destroyed) == 1 {
		return NewInitError("Apollo plugin has been destroyed")
	}
	return nil
}

// setInitialized atomically sets initialization status
func (p *PlugApollo) setInitialized() {
	atomic.StoreInt32(&p.initialized, 1)
}

// setDestroyed atomically sets destruction status
func (p *PlugApollo) setDestroyed() {
	atomic.StoreInt32(&p.destroyed, 1)
}

// StartupTasks implements custom startup logic for the Apollo plugin.
// This function configures and starts the Apollo configuration center, adding necessary middleware and configuration options.
func (p *PlugApollo) StartupTasks() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if atomic.LoadInt32(&p.initialized) == 1 {
		return NewInitError("Apollo plugin already initialized")
	}

	// Record startup operation metrics
	if p.metrics != nil {
		p.metrics.RecordClientOperation("startup", "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordClientOperation("startup", "success")
			}
		}()
	}

	// Use Lynx application Helper to log Apollo plugin initialization information.
	log.Infof("Initializing apollo plugin with app_id: %s, cluster: %s, namespace: %s", p.conf.AppId, p.conf.Cluster, p.conf.Namespace)

	// Initialize Apollo client
	client, err := p.initApolloClient()
	if err != nil {
		log.Errorf("Failed to initialize Apollo client: %v", err)
		if p.metrics != nil {
			p.metrics.RecordClientOperation("startup", "error")
		}
		return WrapInitError(err, "failed to initialize Apollo client")
	}

	// Save client instance
	p.client = client

	// Set the Apollo configuration center as the Lynx application's control plane.
	err = app.Lynx().SetControlPlane(p)
	if err != nil {
		log.Errorf("Failed to set control plane: %v", err)
		if p.metrics != nil {
			p.metrics.RecordClientOperation("startup", "error")
		}
		return WrapInitError(err, "failed to set control plane")
	}

	// Get the Lynx application's control plane startup configuration.
	cfg, err := app.Lynx().InitControlPlaneConfig()
	if err != nil {
		log.Errorf("Failed to init control plane config: %v", err)
		if p.metrics != nil {
			p.metrics.RecordClientOperation("startup", "error")
		}
		return WrapInitError(err, "failed to init control plane config")
	}

	// Load plugins from the plugin list.
	app.Lynx().GetPluginManager().LoadPlugins(cfg)

	p.setInitialized()
	log.Infof("Apollo plugin initialized successfully")
	return nil
}

// initApolloClient initializes Apollo HTTP client
func (p *PlugApollo) initApolloClient() (*ApolloHTTPClient, error) {
	if p.conf.MetaServer == "" {
		return nil, fmt.Errorf("meta server address is required")
	}
	if p.conf.AppId == "" {
		return nil, fmt.Errorf("app ID is required")
	}

	// Set defaults
	cluster := p.conf.Cluster
	if cluster == "" {
		cluster = conf.DefaultCluster
	}

	namespace := p.conf.Namespace
	if namespace == "" {
		namespace = conf.DefaultNamespace
	}

	// Get timeout
	timeout := 10 * time.Second
	if p.conf.Timeout != nil {
		timeout = p.conf.Timeout.AsDuration()
	}

	// Create HTTP client
	client := NewApolloHTTPClient(
		p.conf.MetaServer,
		p.conf.AppId,
		cluster,
		namespace,
		p.conf.Token,
		timeout,
	)

	log.Infof("Apollo HTTP client initialized - MetaServer: %s, AppId: %s, Cluster: %s, Namespace: %s",
		p.conf.MetaServer, p.conf.AppId, cluster, namespace)

	return client, nil
}

// GetMetrics gets monitoring metrics
func (p *PlugApollo) GetMetrics() *Metrics {
	return p.metrics
}

// IsInitialized checks if initialized
func (p *PlugApollo) IsInitialized() bool {
	return atomic.LoadInt32(&p.initialized) == 1
}

// IsDestroyed checks if destroyed
func (p *PlugApollo) IsDestroyed() bool {
	return atomic.LoadInt32(&p.destroyed) == 1
}

// GetApolloConfig gets Apollo configuration
func (p *PlugApollo) GetApolloConfig() *conf.Apollo {
	return p.conf
}

// GetNamespace returns the namespace
func (p *PlugApollo) GetNamespace() string {
	if p.conf != nil {
		return p.conf.Namespace
	}
	return conf.DefaultNamespace
}

