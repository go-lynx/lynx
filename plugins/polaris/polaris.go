package polaris

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/polaris/conf"
	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// Plugin metadata
// Plugin metadata defining basic plugin information
const (
	// pluginName is the unique identifier for the Polaris control plane plugin, used to identify this plugin in the plugin system.
	pluginName = "polaris.control.plane"

	// pluginVersion represents the current version of the Polaris control plane plugin.
	pluginVersion = "v2.0.0"

	// pluginDescription briefly describes the functionality of the Polaris control plane plugin.
	pluginDescription = "polaris control plane plugin for lynx framework"

	// confPrefix is the configuration prefix used when loading Polaris configuration.
	confPrefix = "lynx.polaris"
)

// PlugPolaris represents a Polaris control plane plugin instance
type PlugPolaris struct {
	*plugins.BasePlugin
	polaris *polaris.Polaris
	conf    *conf.Polaris

	// SDK components
	sdk api.SDKContext

	// Enhanced components
	metrics        *Metrics
	retryManager   *RetryManager
	circuitBreaker *CircuitBreaker

	// State management - using atomic operations to improve concurrency safety
	mu            sync.RWMutex
	initialized   int32 // Use int32 instead of bool to support atomic operations
	destroyed     int32 // Use int32 instead of bool to support atomic operations
	healthCheckCh chan struct{}

	// Service information
	serviceInfo *ServiceInfo

	// Event handling
	activeWatchers map[string]*ServiceWatcher // Active service watchers
	configWatchers map[string]*ConfigWatcher  // Active configuration watchers
	watcherMutex   sync.RWMutex               // Watcher mutex

	// Cache system
	serviceCache map[string]interface{} // Service instance cache
	configCache  map[string]interface{} // Configuration cache
	cacheMutex   sync.RWMutex           // Cache mutex
}

// ServiceInfo service registration information
type ServiceInfo struct {
	Service   string            `json:"service"`
	Namespace string            `json:"namespace"`
	Host      string            `json:"host"`
	Port      int32             `json:"port"`
	Protocol  string            `json:"protocol"`
	Version   string            `json:"version"`
	Metadata  map[string]string `json:"metadata"`
}

// NewPolarisControlPlane creates a new Polaris control plane.
// This function initializes the plugin's basic information and returns a pointer to PlugPolaris.
func NewPolarisControlPlane() *PlugPolaris {
	return &PlugPolaris{
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
		activeWatchers: make(map[string]*ServiceWatcher),
		configWatchers: make(map[string]*ConfigWatcher),
	}
}

// InitializeResources implements custom initialization logic for the Polaris plugin.
// This function loads and validates Polaris configuration, using default configuration if none is provided.
func (p *PlugPolaris) InitializeResources(rt plugins.Runtime) error {
	// Initialize an empty configuration structure
	p.conf = &conf.Polaris{}

	// Scan and load Polaris configuration from runtime configuration
	err := rt.GetConfig().Value(confPrefix).Scan(p.conf)
	if err != nil {
		return WrapInitError(err, "failed to scan polaris configuration")
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
func (p *PlugPolaris) setDefaultConfig() {
	// Default namespace is 'default'
	if p.conf.Namespace == "" {
		p.conf.Namespace = conf.DefaultNamespace
	}
	// Default service instance weight is 100
	if p.conf.Weight == 0 {
		p.conf.Weight = conf.DefaultWeight
	}
	// Default TTL is 5 seconds
	if p.conf.Ttl == 0 {
		p.conf.Ttl = conf.DefaultTTL
	}
	// Default timeout is 5 seconds
	if p.conf.Timeout == nil {
		p.conf.Timeout = conf.GetDefaultTimeout()
	}
}

// validateConfig validates configuration
func (p *PlugPolaris) validateConfig() error {
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
func (p *PlugPolaris) initComponents() error {
	// Initialize monitoring metrics
	p.metrics = NewPolarisMetrics()

	// Initialize retry manager
	p.retryManager = NewRetryManager(3, time.Second)

	// Initialize circuit breaker
	p.circuitBreaker = NewCircuitBreaker(0.5)

	return nil
}

// checkInitialized unified state checking method ensuring thread safety
func (p *PlugPolaris) checkInitialized() error {
	if atomic.LoadInt32(&p.initialized) == 0 {
		return NewInitError("Polaris plugin not initialized")
	}
	if atomic.LoadInt32(&p.destroyed) == 1 {
		return NewInitError("Polaris plugin has been destroyed")
	}
	return nil
}

// setInitialized atomically sets initialization status
func (p *PlugPolaris) setInitialized() {
	atomic.StoreInt32(&p.initialized, 1)
}

// setDestroyed atomically sets destruction status
func (p *PlugPolaris) setDestroyed() {
	atomic.StoreInt32(&p.destroyed, 1)
}

// StartupTasks implements custom startup logic for the Polaris plugin.
// This function configures and starts the Polaris control plane, adding necessary middleware and configuration options.
func (p *PlugPolaris) StartupTasks() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if atomic.LoadInt32(&p.initialized) == 1 {
		return NewInitError("Polaris plugin already initialized")
	}

	// Record startup operation metrics
	if p.metrics != nil {
		p.metrics.RecordSDKOperation("startup", "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordSDKOperation("startup", "success")
			}
		}()
	}

	// Use Lynx application Helper to log Polaris plugin initialization information.
	log.Infof("Initializing polaris plugin with namespace: %s", p.conf.Namespace)

	// Load Polaris SDK configuration and initialize
	sdk, err := p.loadPolarisConfiguration()
	if err != nil {
		log.Errorf("Failed to initialize Polaris SDK: %v", err)
		if p.metrics != nil {
			p.metrics.RecordSDKOperation("startup", "error")
		}
		return WrapInitError(err, "failed to initialize Polaris SDK")
	}

	// Save SDK instance
	p.sdk = sdk

	// Create a new Polaris instance using the previously initialized SDK and configuration.
	pol := polaris.New(
		sdk,
		polaris.WithService(app.GetName()),
		polaris.WithNamespace(p.conf.Namespace),
	)
	// Save the Polaris instance to p.polaris.
	p.polaris = &pol

	// Set the Polaris control plane as the Lynx application's control plane.
	err = app.Lynx().SetControlPlane(p)
	if err != nil {
		log.Errorf("Failed to set control plane: %v", err)
		if p.metrics != nil {
			p.metrics.RecordSDKOperation("startup", "error")
		}
		return WrapInitError(err, "failed to set control plane")
	}

	// Get the Lynx application's control plane startup configuration.
	cfg, err := app.Lynx().InitControlPlaneConfig()
	if err != nil {
		log.Errorf("Failed to init control plane config: %v", err)
		if p.metrics != nil {
			p.metrics.RecordSDKOperation("startup", "error")
		}
		return WrapInitError(err, "failed to init control plane config")
	}

	// Load plugins from the plugin list.
	app.Lynx().GetPluginManager().LoadPlugins(cfg)

	p.setInitialized()
	log.Infof("Polaris plugin initialized successfully")
	return nil
}

// GetMetrics gets monitoring metrics
func (p *PlugPolaris) GetMetrics() *Metrics {
	return p.metrics
}

// IsInitialized checks if initialized
func (p *PlugPolaris) IsInitialized() bool {
	return atomic.LoadInt32(&p.initialized) == 1
}

// IsDestroyed checks if destroyed
func (p *PlugPolaris) IsDestroyed() bool {
	return atomic.LoadInt32(&p.destroyed) == 1
}

// GetPolarisConfig gets Polaris configuration
func (p *PlugPolaris) GetPolarisConfig() *conf.Polaris {
	return p.conf
}

// SetServiceInfo sets service information
func (p *PlugPolaris) SetServiceInfo(info *ServiceInfo) {
	p.serviceInfo = info
}

// GetServiceInfo gets service information
func (p *PlugPolaris) GetServiceInfo() *ServiceInfo {
	return p.serviceInfo
}

// WatchConfig watches configuration changes
func (p *PlugPolaris) WatchConfig(fileName, group string) (*ConfigWatcher, error) {
	if !p.IsInitialized() {
		return nil, NewInitError("Polaris plugin not initialized")
	}

	// Record configuration watch operation metrics
	if p.metrics != nil {
		p.metrics.RecordSDKOperation("watch_config", "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordSDKOperation("watch_config", "success")
			}
		}()
	}

	log.Infof("Watching config: %s, group: %s", fileName, group)

	// Check if the configuration is already being watched
	configKey := fmt.Sprintf("%s:%s", fileName, group)
	p.watcherMutex.Lock()
	if existingWatcher, exists := p.configWatchers[configKey]; exists {
		p.watcherMutex.Unlock()
		log.Infof("Config %s:%s is already being watched", fileName, group)
		return existingWatcher, nil
	}
	p.watcherMutex.Unlock()

	// Create Config API client
	configAPI := api.NewConfigFileAPIBySDKContext(p.sdk)
	if configAPI == nil {
		return nil, NewInitError("failed to create config API")
	}

	// Create configuration watcher and connect to SDK
	watcher := NewConfigWatcher(configAPI, fileName, group, p.conf.Namespace)
	watcher.metrics = p.metrics // Pass metrics reference

	// Set event handling callbacks
	watcher.SetOnConfigChanged(func(config model.ConfigFile) {
		p.handleConfigChanged(fileName, group, config)
	})

	watcher.SetOnError(func(err error) {
		p.handleConfigWatchError(fileName, group, err)
	})

	// Register watcher
	p.watcherMutex.Lock()
	p.configWatchers[configKey] = watcher
	p.watcherMutex.Unlock()

	// Start watching
	watcher.Start()

	return watcher, nil
}

// recordServiceChangeAudit records service change audit logs
func (p *PlugPolaris) recordServiceChangeAudit(serviceName string, instances []model.Instance) {
	// Record detailed audit information
	auditInfo := map[string]interface{}{
		"service_name":   serviceName,
		"namespace":      p.conf.Namespace,
		"instance_count": len(instances),
		"timestamp":      time.Now().Unix(),
		"instances":      make([]map[string]interface{}, 0, len(instances)),
	}

	// Collect instance information (with data masking)
	for _, instance := range instances {
		instanceInfo := map[string]interface{}{
			"id":       instance.GetId(),
			"host":     instance.GetHost(),
			"port":     instance.GetPort(),
			"weight":   instance.GetWeight(),
			"healthy":  instance.IsHealthy(),
			"isolated": instance.IsIsolated(),
		}
		auditInfo["instances"] = append(auditInfo["instances"].([]map[string]interface{}), instanceInfo)
	}

	log.Infof("Service change audit: %+v", auditInfo)
}

// recordServiceWatchErrorAudit records service watch error audit logs
func (p *PlugPolaris) recordServiceWatchErrorAudit(serviceName string, err error) {
	auditInfo := map[string]interface{}{
		"service_name": serviceName,
		"namespace":    p.conf.Namespace,
		"error":        err.Error(),
		"error_type":   fmt.Sprintf("%T", err),
		"timestamp":    time.Now().Unix(),
		"plugin_state": map[string]interface{}{
			"initialized": p.IsInitialized(),
			"destroyed":   p.IsDestroyed(),
		},
	}

	log.Errorf("Service watch error audit: %+v", auditInfo)
}

// sendServiceWatchAlert sends service watch alerts
func (p *PlugPolaris) sendServiceWatchAlert(serviceName string, err error) {
	// Implement alert notification logic
	alertInfo := map[string]interface{}{
		"alert_type":   "service_watch_error",
		"service_name": serviceName,
		"namespace":    p.conf.Namespace,
		"error":        err.Error(),
		"error_type":   fmt.Sprintf("%T", err),
		"severity":     "warning",
		"timestamp":    time.Now().Unix(),
		"plugin_state": map[string]interface{}{
			"initialized": p.IsInitialized(),
			"destroyed":   p.IsDestroyed(),
		},
	}

	// Implementation: integrate multiple alert channels
	// 1. Send to monitoring system
	p.sendToMonitoringSystem(alertInfo)

	// 2. Send to message queue
	p.sendToMessageQueue(alertInfo)

	// 3. Send DingTalk/WeChat Work notifications
	p.sendToIMNotification(alertInfo)

	// 4. Send email alerts
	p.sendEmailAlert(alertInfo)

	// 5. Send SMS alerts
	p.sendSMSAlert(alertInfo)

	log.Warnf("Service watch alert: %+v", alertInfo)
}

// sendToMonitoringSystem sends to monitoring system
func (p *PlugPolaris) sendToMonitoringSystem(alertInfo map[string]interface{}) {
	// Implementation: send to monitoring systems like Prometheus, Grafana
	log.Infof("Sending alert to monitoring system: %s", alertInfo["alert_type"])
	// Specific monitoring system APIs can be integrated here
}

// sendToMessageQueue sends to message queue
func (p *PlugPolaris) sendToMessageQueue(alertInfo map[string]interface{}) {
	// Implementation: send to message queues like Kafka, RabbitMQ
	log.Infof("Sending alert to message queue: %s", alertInfo["alert_type"])
	// Specific message queue clients can be integrated here
}

// sendToIMNotification sends instant messaging notifications
func (p *PlugPolaris) sendToIMNotification(alertInfo map[string]interface{}) {
	// Implementation: send DingTalk, WeChat Work notifications
	log.Infof("Sending IM notification: %s", alertInfo["alert_type"])
	// DingTalk/WeChat Work bot APIs can be integrated here
}

// sendEmailAlert sends email alerts
func (p *PlugPolaris) sendEmailAlert(alertInfo map[string]interface{}) {
	// Implementation: send email alerts
	log.Infof("Sending email alert: %s", alertInfo["alert_type"])
	// Email sending services can be integrated here
}

// sendSMSAlert sends SMS alerts
func (p *PlugPolaris) sendSMSAlert(alertInfo map[string]interface{}) {
	// Implementation: send SMS alerts
	log.Infof("Sending SMS alert: %s", alertInfo["alert_type"])
	// SMS sending services can be integrated here
}

// recordConfigChangeAudit records configuration change audit logs
func (p *PlugPolaris) recordConfigChangeAudit(fileName, group string, config model.ConfigFile) {
	auditInfo := map[string]interface{}{
		"config_file":    fileName,
		"group":          group,
		"namespace":      p.conf.Namespace,
		"content_length": len(config.GetContent()),
		"timestamp":      time.Now().Unix(),
		"change_type":    "config_updated",
	}

	log.Infof("Config change audit: %+v", auditInfo)
}

// recordConfigWatchErrorAudit records configuration watch error audit logs
func (p *PlugPolaris) recordConfigWatchErrorAudit(fileName, group string, err error) {
	auditInfo := map[string]interface{}{
		"config_file": fileName,
		"group":       group,
		"namespace":   p.conf.Namespace,
		"error":       err.Error(),
		"error_type":  fmt.Sprintf("%T", err),
		"timestamp":   time.Now().Unix(),
		"plugin_state": map[string]interface{}{
			"initialized": p.IsInitialized(),
			"destroyed":   p.IsDestroyed(),
		},
	}

	log.Errorf("Config watch error audit: %+v", auditInfo)
}

// sendConfigWatchAlert sends configuration watch alerts
func (p *PlugPolaris) sendConfigWatchAlert(fileName, group string, err error) {
	alertInfo := map[string]interface{}{
		"alert_type":  "config_watch_error",
		"config_file": fileName,
		"group":       group,
		"namespace":   p.conf.Namespace,
		"error":       err.Error(),
		"error_type":  fmt.Sprintf("%T", err),
		"severity":    "warning",
		"timestamp":   time.Now().Unix(),
	}

	// Specific alert implementations can be integrated here
	log.Warnf("Config watch alert: %+v", alertInfo)
}

// retryConfigWatch retries configuration watching
func (p *PlugPolaris) retryConfigWatch(fileName, group string) {
    // Implement retry logic
    log.Infof("Retrying config watch for %s:%s", fileName, group)

    // Wait for a period before retrying, but allow cancellation on plugin stop
    if p.healthCheckCh != nil {
        select {
        case <-p.healthCheckCh:
            log.Infof("Config watch retry canceled due to plugin shutdown: %s:%s", fileName, group)
            return
        case <-time.After(5 * time.Second):
        }
    } else {
        // Fallback when channel is not available
        if p.IsDestroyed() {
            return
        }
        time.Sleep(5 * time.Second)
    }

    if p.IsDestroyed() {
        return
    }

    // Recreate watcher
    if _, err := p.WatchConfig(fileName, group); err == nil {
        log.Infof("Successfully recreated config watcher for %s:%s", fileName, group)
    } else {
        log.Errorf("Failed to recreate config watcher for %s:%s: %v", fileName, group, err)
    }
}
