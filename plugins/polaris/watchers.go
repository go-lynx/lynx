package polaris

import (
	"context"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// ServiceWatcher and ConfigWatcher modules
// Responsibility: underlying service change monitoring and configuration change monitoring
// Difference from registry_impl.go:
// - watchers.go: underlying monitoring capabilities, directly interacts with Polaris SDK
// - registry_impl.go: Kratos framework adaptation, implements registry interface

// ServiceWatcher service watcher
// Monitors service instance changes
type ServiceWatcher struct {
	consumer    api.ConsumerAPI
	serviceName string
	namespace   string

	// Monitoring control
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
	wg     sync.WaitGroup // Add WaitGroup to ensure goroutine exits correctly

	// Callback functions
	onInstancesChanged func(instances []model.Instance)
	onError            func(error)

	// State
	isRunning     bool
	lastInstances []model.Instance

	// Monitoring metrics
	metrics *Metrics
}

// NewServiceWatcher creates new service watcher
func NewServiceWatcher(consumer api.ConsumerAPI, serviceName, namespace string) *ServiceWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &ServiceWatcher{
		consumer:    consumer,
		serviceName: serviceName,
		namespace:   namespace,
		ctx:         ctx,
		cancel:      cancel,
		metrics:     nil, // Will be set when used
	}
}

// SetOnInstancesChanged sets instance change callback
func (sw *ServiceWatcher) SetOnInstancesChanged(callback func(instances []model.Instance)) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.onInstancesChanged = callback
}

// SetOnError sets error callback
func (sw *ServiceWatcher) SetOnError(callback func(error)) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.onError = callback
}

// Start starts monitoring
func (sw *ServiceWatcher) Start() {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.isRunning {
		return
	}

	// Record monitoring start metrics
	if sw.metrics != nil {
		sw.metrics.RecordSDKOperation("service_watch_start", "success")
	}

	sw.isRunning = true
	sw.wg.Add(1) // Increment WaitGroup count
	go func() {
		defer sw.wg.Done() // Ensure count is decremented when goroutine exits
		sw.watchLoop()
	}()

	log.Infof("Started watching service: %s in namespace: %s", sw.serviceName, sw.namespace)
}

// Stop stops monitoring
func (sw *ServiceWatcher) Stop() {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if !sw.isRunning {
		return
	}

	// Record monitoring stop metrics
	if sw.metrics != nil {
		sw.metrics.RecordSDKOperation("service_watch_stop", "success")
	}

	sw.cancel()
	sw.isRunning = false

	// Wait for goroutine to completely exit
	sw.wg.Wait()

	log.Infof("Stopped watching service: %s", sw.serviceName)
}

// watchLoop monitoring loop
func (sw *ServiceWatcher) watchLoop() {
	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer ticker.Stop()

	for {
		select {
		case <-sw.ctx.Done():
			log.Infof("Watch loop for service %s stopped due to context cancellation", sw.serviceName)
			return
		case <-ticker.C:
			sw.checkInstances()
		}
	}
}

// checkInstances checks instance changes
func (sw *ServiceWatcher) checkInstances() {
	req := &api.GetInstancesRequest{
		GetInstancesRequest: model.GetInstancesRequest{
			Service:   sw.serviceName,
			Namespace: sw.namespace,
		},
	}

	resp, err := sw.consumer.GetInstances(req)
	if err != nil {
		log.Errorf("Failed to get instances for service %s: %v", sw.serviceName, err)
		sw.notifyError(err)
		return
	}

	// Check if instances have changed
	if sw.hasInstancesChanged(resp.Instances) {
		sw.lastInstances = resp.Instances
		sw.notifyInstancesChanged(resp.Instances)

		log.Infof("Service %s instances changed: %d instances",
			sw.serviceName, len(resp.Instances))
	}
}

// hasInstancesChanged checks if instances have changed
func (sw *ServiceWatcher) hasInstancesChanged(newInstances []model.Instance) bool {
	// If instance count is different, consider it changed
	if len(sw.lastInstances) != len(newInstances) {
		return true
	}

	// If there were no instances before, but now there are, consider it changed
	if len(sw.lastInstances) == 0 && len(newInstances) > 0 {
		return true
	}

	// If there were instances before, but now there are none, consider it changed
	if len(sw.lastInstances) > 0 && len(newInstances) == 0 {
		return true
	}

	// If instance count is the same, perform detailed comparison
	lastInstancesMap := make(map[string]model.Instance)
	for _, instance := range sw.lastInstances {
		key := instance.GetId()
		lastInstancesMap[key] = instance
	}

	// Check each new instance
	for _, newInstance := range newInstances {
		key := newInstance.GetId()
		lastInstance, exists := lastInstancesMap[key]

		if !exists {
			// Found new instance
			return true
		}

		// Compare instance properties
		if !sw.compareInstance(lastInstance, newInstance) {
			// Instance properties have changed
			return true
		}

		// Remove compared instance from map
		delete(lastInstancesMap, key)
	}

	// If there are remaining old instances, it means instances were removed
	if len(lastInstancesMap) > 0 {
		return true
	}

	return false
}

// notifyInstancesChanged notifies instance changes
func (sw *ServiceWatcher) notifyInstancesChanged(instances []model.Instance) {
	// Record instance change metrics
	if sw.metrics != nil {
		sw.metrics.RecordServiceDiscovery(sw.serviceName, sw.namespace, "changed")
	}

	sw.mu.RLock()
	defer sw.mu.RUnlock()

	if sw.onInstancesChanged != nil {
		sw.onInstancesChanged(instances)
	}
}

// notifyError notifies error
func (sw *ServiceWatcher) notifyError(err error) {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	if sw.onError != nil {
		sw.onError(err)
	}
}

// GetLastInstances gets the last instance list
func (sw *ServiceWatcher) GetLastInstances() []model.Instance {
	sw.mu.RLock()
	defer sw.mu.RUnlock()
	return sw.lastInstances
}

// IsRunning checks if it's running
func (sw *ServiceWatcher) IsRunning() bool {
	sw.mu.RLock()
	defer sw.mu.RUnlock()
	return sw.isRunning
}

// ConfigWatcher configuration watcher
// Monitors configuration changes
type ConfigWatcher struct {
	configAPI api.ConfigFileAPI
	fileName  string
	group     string
	namespace string

	// Monitoring control
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
	wg     sync.WaitGroup // Add WaitGroup to ensure goroutine exits correctly

	// Callback functions
	onConfigChanged func(config model.ConfigFile)
	onError         func(error)

	// State
	isRunning  bool
	lastConfig model.ConfigFile

	// Monitoring metrics
	metrics *Metrics
}

// NewConfigWatcher creates new configuration watcher
func NewConfigWatcher(configAPI api.ConfigFileAPI, fileName, group, namespace string) *ConfigWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &ConfigWatcher{
		configAPI: configAPI,
		fileName:  fileName,
		group:     group,
		namespace: namespace,
		ctx:       ctx,
		cancel:    cancel,
		metrics:   nil, // Will be set when used
	}
}

// SetOnConfigChanged sets configuration change callback
func (cw *ConfigWatcher) SetOnConfigChanged(callback func(config model.ConfigFile)) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.onConfigChanged = callback
}

// SetOnError sets error callback
func (cw *ConfigWatcher) SetOnError(callback func(error)) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.onError = callback
}

// Start starts monitoring
func (cw *ConfigWatcher) Start() {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if cw.isRunning {
		return
	}

	// Record monitoring start metrics
	if cw.metrics != nil {
		cw.metrics.RecordSDKOperation("config_watch_start", "success")
	}

	cw.isRunning = true
	cw.wg.Add(1) // Increment WaitGroup count
	go func() {
		defer cw.wg.Done() // Ensure count is decremented when goroutine exits
		cw.watchLoop()
	}()

	log.Infof("Started watching config: %s:%s in namespace: %s", cw.fileName, cw.group, cw.namespace)
}

// Stop stops monitoring
func (cw *ConfigWatcher) Stop() {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if !cw.isRunning {
		return
	}

	// Record monitoring stop metrics
	if cw.metrics != nil {
		cw.metrics.RecordSDKOperation("config_watch_stop", "success")
	}

	cw.cancel()
	cw.isRunning = false

	// Wait for goroutine to completely exit
	cw.wg.Wait()

	log.Infof("Stopped watching config: %s:%s", cw.fileName, cw.group)
}

// watchLoop monitoring loop
func (cw *ConfigWatcher) watchLoop() {
	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer ticker.Stop()

	for {
		select {
		case <-cw.ctx.Done():
			log.Infof("Watch loop for config %s:%s stopped due to context cancellation", cw.fileName, cw.group)
			return
		case <-ticker.C:
			cw.checkConfig()
		}
	}
}

// checkConfig checks configuration changes
func (cw *ConfigWatcher) checkConfig() {
	// Record configuration check operation metrics
	if cw.metrics != nil {
		cw.metrics.RecordConfigOperation("check", cw.fileName, cw.group, "start")
		defer func() {
			if cw.metrics != nil {
				cw.metrics.RecordConfigOperation("check", cw.fileName, cw.group, "success")
			}
		}()
	}

	config, err := cw.configAPI.GetConfigFile(cw.namespace, cw.group, cw.fileName)
	if err != nil {
		log.Errorf("Failed to get config %s:%s: %v", cw.group, cw.fileName, err)
		if cw.metrics != nil {
			cw.metrics.RecordConfigOperation("check", cw.fileName, cw.group, "error")
		}
		cw.notifyError(err)
		return
	}

	// Check if configuration has changed
	if cw.hasConfigChanged(config) {
		cw.lastConfig = config
		cw.notifyConfigChanged(config)

		log.Infof("Config %s:%s changed",
			cw.group, cw.fileName)
	}
}

// hasConfigChanged checks if configuration has changed
func (cw *ConfigWatcher) hasConfigChanged(newConfig model.ConfigFile) bool {
	// If there was no configuration before, but now there is, consider it changed
	if cw.lastConfig == nil && newConfig != nil {
		return true
	}

	// If there was configuration before, but now there is none, consider it changed
	if cw.lastConfig != nil && newConfig == nil {
		return true
	}

	// If both configurations are nil, consider it unchanged
	if cw.lastConfig == nil && newConfig == nil {
		return false
	}

	// Compare configuration namespace
	if cw.lastConfig.GetNamespace() != newConfig.GetNamespace() {
		return true
	}

	// Compare configuration file group
	if cw.lastConfig.GetFileGroup() != newConfig.GetFileGroup() {
		return true
	}

	// Compare configuration file name
	if cw.lastConfig.GetFileName() != newConfig.GetFileName() {
		return true
	}

	// Compare configuration content
	if cw.lastConfig.GetContent() != newConfig.GetContent() {
		return true
	}

	// Compare if there is content
	if cw.lastConfig.HasContent() != newConfig.HasContent() {
		return true
	}

	return false
}

// notifyConfigChanged notifies configuration changes
func (cw *ConfigWatcher) notifyConfigChanged(config model.ConfigFile) {
	// Record configuration change metrics
	if cw.metrics != nil {
		cw.metrics.RecordConfigChange(cw.fileName, cw.group)
	}

	cw.mu.RLock()
	defer cw.mu.RUnlock()

	if cw.onConfigChanged != nil {
		cw.onConfigChanged(config)
	}
}

// notifyError notifies error
func (cw *ConfigWatcher) notifyError(err error) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	if cw.onError != nil {
		cw.onError(err)
	}
}

// GetLastConfig gets the last configuration
func (cw *ConfigWatcher) GetLastConfig() model.ConfigFile {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.lastConfig
}

// IsRunning checks if it's running
func (cw *ConfigWatcher) IsRunning() bool {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.isRunning
}

// compareInstance compares if two instances are the same
func (sw *ServiceWatcher) compareInstance(instance1, instance2 model.Instance) bool {
	// Compare basic information
	if instance1.GetId() != instance2.GetId() ||
		instance1.GetHost() != instance2.GetHost() ||
		instance1.GetPort() != instance2.GetPort() ||
		instance1.GetProtocol() != instance2.GetProtocol() ||
		instance1.GetVersion() != instance2.GetVersion() {
		return false
	}

	// Compare weight
	if instance1.GetWeight() != instance2.GetWeight() {
		return false
	}

	// Compare health status
	if instance1.IsHealthy() != instance2.IsHealthy() {
		return false
	}

	// Compare isolation status
	if instance1.IsIsolated() != instance2.IsIsolated() {
		return false
	}

	// Compare metadata
	metadata1 := instance1.GetMetadata()
	metadata2 := instance2.GetMetadata()

	if len(metadata1) != len(metadata2) {
		return false
	}

	for key, value1 := range metadata1 {
		if value2, exists := metadata2[key]; !exists || value1 != value2 {
			return false
		}
	}

	return true
}
