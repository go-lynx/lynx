// Package lynx provides the core application framework for building microservices.
//
// This file (app.go) contains the LynxApp structure and main API entry points.
package lynx

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-lynx/lynx/conf"
	"github.com/go-lynx/lynx/events"
	"github.com/go-lynx/lynx/log"
	"github.com/go-lynx/lynx/subscribe"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-lynx/lynx/plugins"
	"google.golang.org/grpc"
)

var (
	// lynxApp is the singleton instance of the Lynx application
	lynxApp *LynxApp
	// RW mutex protecting reads/writes of lynxApp to avoid race conditions
	lynxMu sync.RWMutex
	// initMu protects initialization state.
	initMu sync.Mutex
	// initErr stores the last initialization error.
	initErr error
	// initCompleted indicates whether at least one initialization attempt finished.
	initCompleted bool
	// initInProgress indicates whether an initialization attempt is currently running.
	initInProgress bool
	// initDone channel signals current initialization attempt completion.
	initDone chan struct{}
)

// SetDefaultApp publishes app as the process-wide default Lynx application instance.
// It intentionally does not manage initialization lifecycle; callers should only use it
// after a fully initialized app is available.
func SetDefaultApp(app *LynxApp) {
	lynxMu.Lock()
	defer lynxMu.Unlock()
	lynxApp = app
	events.SetDefaultEventBusProvider(func() *events.EventBusManager {
		if lynxApp == nil {
			return nil
		}
		return lynxApp.eventManager
	})
	events.SetDefaultListenerManagerProvider(func() *events.EventListenerManager {
		if lynxApp == nil {
			return nil
		}
		return lynxApp.eventListenerManager
	})
}

// ClearDefaultApp clears the process-wide default Lynx application instance.
func ClearDefaultApp() {
	SetDefaultApp(nil)
}

// clearDefaultAppIf clears the global default only when it still points to app.
func clearDefaultAppIf(app *LynxApp) bool {
	lynxMu.Lock()
	defer lynxMu.Unlock()
	if lynxApp != app {
		return false
	}
	lynxApp = nil
	return true
}

// resetInitState resets initialization state (for testing/restart scenarios)
// Should only be called during application shutdown
func resetInitState() {
	initMu.Lock()
	defer initMu.Unlock()
	initErr = nil
	initCompleted = false
	initInProgress = false
	initDone = nil
}

// getInitTimeout returns the initialization timeout from config, default 30s.
// Can be configured via "lynx.app.init_timeout" config key.
func getInitTimeout(cfg config.Config) time.Duration {
	defaultTimeout := 30 * time.Second
	if cfg == nil {
		return defaultTimeout
	}

	var confStr string
	if err := cfg.Value("lynx.app.init_timeout").Scan(&confStr); err == nil {
		if parsed, err2 := time.ParseDuration(confStr); err2 == nil {
			// Validate timeout range: 10s to 300s
			if parsed < 10*time.Second {
				log.Warnf("init_timeout too short (%v), using minimum 10s", parsed)
				return 10 * time.Second
			}
			if parsed > 300*time.Second {
				log.Warnf("init_timeout too long (%v), using maximum 300s", parsed)
				return 300 * time.Second
			}
			return parsed
		}
	}
	return defaultTimeout
}

// Fixed plugin ID used internally for configuration-related events.
// Avoids using an empty string which would break PluginID-based filtering.
const configEventPluginID = "lynx.config"

// LynxApp represents the main application instance.
// It serves as the central coordinator for all application components,
// managing configuration, logging, plugins, and the control plane.
type LynxApp struct {
	// host represents the application's host address.
	// Used for network communication and service registration.
	host string

	// name is the unique identifier of the application.
	// Used for service discovery and logging.
	name string

	// version represents the application's version number.
	// Used for compatibility checks and deployment management.
	version string

	// certificateProvider manages the application's TLS/SSL certificates.
	// Used for secure communication and TLS configuration.
	cert CertificateProvider

	// Bootstrap configuration
	bootConfig *conf.Bootstrap

	// globalConf holds the application's global configuration.
	// Contains settings that apply across all components.
	globalConf config.Config

	// controlPlane manages the application's control interface (full implementation).
	// Handles dynamic configuration updates and system monitoring.
	controlPlane ControlPlane

	// Optional capability overrides for partial control plane implementations.
	// When set, these take precedence over controlPlane for the respective capability.
	// Plugins can register only the capabilities they implement via SetRateLimiter, etc.
	rateLimiter     RateLimiter
	serviceRegistry ServiceRegistry
	routeManager    RouteManager
	configManager   ConfigManager
	systemCore      SystemCore

	// controlPlaneMu protects control plane and capability fields
	controlPlaneMu sync.RWMutex

	// pluginManager handles plugin lifecycle and dependencies.
	// Provides type-safe plugin management with generic support.
	pluginManager TypedPluginManager

	// grpcSubs stores upstream gRPC connections subscribed via configuration; key is the service name
	// Protected by grpcSubsMu for concurrent access
	grpcSubsMu sync.RWMutex
	grpcSubs   map[string]*grpc.ClientConn

	// Configuration version (monotonically increasing) used for event ordering and idempotent handling
	// Uses atomic operations for thread-safe access
	configVersion uint64

	// App-owned event facilities. Core runtime paths should prefer these over global singletons.
	eventManager         *events.EventBusManager
	eventListenerManager *events.EventListenerManager
	eventAdapter         plugins.EventBusAdapter
}

// Lynx returns the global LynxApp instance.
// It ensures thread-safe access to the singleton instance.
func Lynx() *LynxApp {
	lynxMu.RLock()
	defer lynxMu.RUnlock()
	return lynxApp
}

// Host returns the host of this application instance.
func (a *LynxApp) Host() string {
	if a == nil {
		return ""
	}
	return a.host
}

// Name returns the name of this application instance.
func (a *LynxApp) Name() string {
	if a == nil {
		return ""
	}
	return a.name
}

// Version returns the version of this application instance.
func (a *LynxApp) Version() string {
	if a == nil {
		return ""
	}
	return a.version
}

// NewStandaloneApp creates a fully initialized Lynx application instance without
// publishing it as the process-wide default singleton.
//
// This is the preferred constructor for tests, isolated runtimes, and future
// multi-instance scenarios. Call SetDefaultApp explicitly only when a global
// default instance is truly needed.
func NewStandaloneApp(cfg config.Config, plugins ...plugins.Plugin) (*LynxApp, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}
	return initializeApp(cfg, plugins...)
}

// NewApp creates or returns the process-wide default Lynx application instance.
// It preserves the historical singleton behavior for compatibility, while the
// actual instance construction is delegated to NewStandaloneApp/initializeApp.
//
// NewApp uses an explicit initialization state machine instead of sync.Once so
// failed initialization can be retried and concurrent callers can wait on the
// in-flight attempt with a bounded timeout.
func NewApp(cfg config.Config, plugins ...plugins.Plugin) (*LynxApp, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}

	initTimeout := getInitTimeout(cfg)

	for {
		// Fast path: already published default instance.
		if existing := Lynx(); existing != nil {
			log.Warnf("Lynx application already initialized, returning existing instance. New configuration and plugins are ignored.")
			return existing, nil
		}

		initMu.Lock()
		if initInProgress {
			doneChan := initDone
			initMu.Unlock()
			if doneChan == nil {
				return nil, fmt.Errorf("initialization in progress but completion channel is nil")
			}
			select {
			case <-doneChan:
				continue
			case <-time.After(initTimeout):
				return nil, fmt.Errorf("initialization timeout: initialization did not complete within %v", initTimeout)
			}
		}

		// If a previous attempt failed and no new initialization is in progress,
		// allow a fresh retry with the latest cfg/plugins.
		initInProgress = true
		initCompleted = false
		initErr = nil
		doneChan := make(chan struct{})
		initDone = doneChan
		initMu.Unlock()

		app, err := NewStandaloneApp(cfg, plugins...)

		initMu.Lock()
		if err != nil {
			initErr = err
		} else {
			SetDefaultApp(app)
		}
		initCompleted = true
		initInProgress = false
		close(doneChan)
		initMu.Unlock()

		if err != nil {
			return nil, fmt.Errorf("failed to initialize application: %w", err)
		}
		if app == nil {
			return nil, fmt.Errorf("application initialization resulted in nil instance")
		}
		return app, nil
	}
}

// initializeApp handles the actual initialization of the LynxApp instance.
func initializeApp(cfg config.Config, plugins ...plugins.Plugin) (*LynxApp, error) {
	// Parse bootstrap configuration
	var bConf conf.Bootstrap
	// Scan configuration into bConf; return error if scanning fails
	if err := cfg.Scan(&bConf); err != nil {
		return nil, fmt.Errorf("failed to parse bootstrap configuration: %w", err)
	}

	// Validate bootstrap configuration
	if bConf.Lynx == nil || bConf.Lynx.Application == nil {
		return nil, fmt.Errorf("invalid bootstrap configuration: missing required fields")
	}

	// Get host from configuration, fallback to system hostname if not configured
	host := bConf.Lynx.Application.Host
	if host == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("failed to get hostname: %w", err)
		}
		host = hostname
	}

	// Create new application instance
	typedMgr := NewTypedPluginManager(plugins...)
	typedMgr.SetConfig(cfg)
	eventManager, err := events.NewEventBusManager(events.DefaultBusConfigs())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize app event system: %w", err)
	}
	eventListenerManager := events.NewEventListenerManagerWithEventBus(eventManager)
	eventAdapter := events.NewPluginEventBusAdapterWithListenerManager(eventManager, eventListenerManager)
	app := &LynxApp{
		host:                 host,
		name:                 bConf.Lynx.Application.Name,
		version:              bConf.Lynx.Application.Version,
		bootConfig:           &bConf,
		globalConf:           cfg,
		pluginManager:        typedMgr,
		controlPlane:         &DefaultControlPlane{},
		grpcSubs:             make(map[string]*grpc.ClientConn),
		eventManager:         eventManager,
		eventListenerManager: eventListenerManager,
		eventAdapter:         eventAdapter,
	}
	app.eventManager.StartHealthCheck(30 * time.Second)
	app.injectRuntimeEventAdapter()

	// Validate required fields
	if app.name == "" {
		return nil, fmt.Errorf("application name cannot be empty")
	}

	// Emit system start event
	app.emitSystemEvent(events.EventSystemStart, map[string]any{
		"app_name":    app.name,
		"app_version": app.version,
		"host":        app.host,
	})

	return app, nil
}

// GetPluginManager returns the plugin manager instance.
// Returns nil if the application is not initialized.
func (a *LynxApp) GetPluginManager() TypedPluginManager {
	if a == nil {
		return nil
	}
	return a.pluginManager
}

// GetTypedPluginManager returns the typed plugin manager instance.
// Returns nil if the application is not initialized.
// This is an alias for GetPluginManager() for backward compatibility.
func (a *LynxApp) GetTypedPluginManager() TypedPluginManager {
	return a.GetPluginManager()
}

// GetGlobalConfig returns the global configuration instance.
// Returns nil if the application is not initialized.
func (a *LynxApp) GetGlobalConfig() config.Config {
	if a == nil {
		return nil
	}
	return a.globalConf
}

// GetTypedPlugin globally retrieves a type-safe plugin instance
func GetTypedPlugin[T plugins.Plugin](name string) (T, error) {
	var zero T
	a := Lynx()
	if a == nil {
		return zero, fmt.Errorf("lynx application not initialized")
	}

	manager := a.GetTypedPluginManager()
	if manager == nil {
		return zero, fmt.Errorf("typed plugin manager not initialized")
	}

	// Retrieve via the unified TypedPluginManager and perform a type assertion
	return GetTypedPluginFromManager[T](manager, name)
}

// SetGlobalConfig updates the global configuration instance.
// It properly closes the existing configuration before updating.
func (a *LynxApp) SetGlobalConfig(cfg config.Config) error {
	// Check if application instance is nil
	if a == nil {
		return fmt.Errorf("application instance is nil")
	}

	// Validate the new configuration is not nil
	if cfg == nil {
		return fmt.Errorf("new configuration cannot be nil")
	}

	oldCfg := a.globalConf
	pm := a.GetPluginManager()
	var rt plugins.Runtime
	if pm != nil {
		rt = pm.GetRuntime()
	}

	ver := a.nextConfigVersion()
	if pm != nil {
		plan := pm.GetConfigReloadPlan()
		if len(plan.Invalid) > 0 {
			err := fmt.Errorf("configuration protocol invalid for plugins: %v", configReloadEntryNames(plan.Invalid))
			a.emitConfigEvent(rt, plugins.EventConfigurationInvalid, ver, 1, map[string]any{
				"phase":   "plan",
				"invalid": configReloadEntryNames(plan.Invalid),
				"error":   err.Error(),
			})
			return err
		}
		if len(plan.RestartRequired) > 0 {
			err := fmt.Errorf("configuration update requires restart for plugins: %v", configReloadEntryNames(plan.RestartRequired))
			a.emitConfigEvent(rt, plugins.EventConfigurationInvalid, ver, 1, map[string]any{
				"phase":            "plan",
				"restart_required": configReloadEntryNames(plan.RestartRequired),
				"error":            err.Error(),
			})
			return err
		}
	}
	a.emitConfigEvent(rt, plugins.EventConfigurationChanged, ver, 1, map[string]any{
		"phase": "validate",
	})

	configurablePlugins := a.listConfigurablePlugins(pm)
	if err := a.validatePluginConfigurations(configurablePlugins, cfg); err != nil {
		a.emitConfigEvent(rt, plugins.EventConfigurationInvalid, ver, 2, map[string]any{
			"phase": "validate",
			"error": err.Error(),
		})
		return err
	}

	if pm != nil {
		pm.SetConfig(cfg)
	}
	if rt != nil {
		rt.SetConfig(cfg)
	}
	a.globalConf = cfg

	applied, err := a.applyPluginConfigurations(configurablePlugins, cfg)
	if err != nil {
		rollbackErr := a.rollbackPluginConfigurations(applied, oldCfg)
		if pm != nil {
			pm.SetConfig(oldCfg)
		}
		if rt != nil {
			rt.SetConfig(oldCfg)
		}
		a.globalConf = oldCfg
		if cfg != nil {
			_ = cfg.Close()
		}

		a.emitConfigEvent(rt, plugins.EventRollbackInitiated, ver, 3, map[string]any{
			"phase":            "rollback",
			"applied_count":    len(applied),
			"failed_plugin":    err.Plugin.Name(),
			"failed_plugin_id": err.Plugin.ID(),
		})
		if rollbackErr != nil {
			a.emitConfigEvent(rt, plugins.EventRollbackFailed, ver, 4, map[string]any{
				"phase": "rollback",
				"error": rollbackErr.Error(),
			})
			return fmt.Errorf("apply config failed for plugin %s and rollback failed: %w", err.Plugin.Name(), rollbackErr)
		}
		a.emitConfigEvent(rt, plugins.EventRollbackCompleted, ver, 4, map[string]any{
			"phase":         "rollback",
			"applied_count": len(applied),
		})
		return fmt.Errorf("apply config failed for plugin %s: %w", err.Plugin.Name(), err.Err)
	}

	a.emitConfigEvent(rt, plugins.EventConfigurationApplied, ver, 2, map[string]any{
		"phase":        "apply",
		"plugin_count": len(applied),
		"acknowledged": len(applied),
	})

	if oldCfg != nil {
		if err := oldCfg.Close(); err != nil {
			log.Errorf("Failed to close existing configuration: %v", err)
			return err
		}
	}

	return nil
}

type configApplyError struct {
	Plugin plugins.Plugin
	Err    error
}

type runtimeConfigPlugin struct {
	Plugin       plugins.Plugin
	Configurable plugins.Configurable
	Validator    plugins.ConfigValidator
	Rollbacker   plugins.ConfigRollbacker
	Caps         plugins.PluginCapabilities
}

func configReloadEntryNames(entries []ConfigReloadEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.PluginName)
	}
	return names
}

func (a *LynxApp) nextConfigVersion() uint64 {
	var ver uint64
	for {
		oldVer := atomic.LoadUint64(&a.configVersion)
		newVer := oldVer + 1
		if newVer == 0 {
			newVer = 1
		}
		if atomic.CompareAndSwapUint64(&a.configVersion, oldVer, newVer) {
			ver = newVer
			if oldVer == ^uint64(0) {
				log.Warnf("Configuration version overflow detected, resetting to 1. This should not happen in normal operation.")
			}
			break
		}
		runtime.Gosched()
	}
	return ver
}

func (a *LynxApp) emitConfigEvent(rt plugins.Runtime, eventType plugins.EventType, version uint64, sequence int, extra map[string]any) {
	if rt == nil {
		return
	}
	metadata := map[string]any{
		"app":            a.name,
		"version":        a.version,
		"host":           a.host,
		"source":         "SetGlobalConfig",
		"config_version": version,
		"event_sequence": sequence,
	}
	for k, v := range extra {
		metadata[k] = v
	}
	rt.EmitEvent(plugins.PluginEvent{
		Type:      eventType,
		Priority:  plugins.PriorityHigh,
		Source:    "SetGlobalConfig",
		Category:  "configuration",
		PluginID:  configEventPluginID,
		Status:    plugins.StatusActive,
		Timestamp: time.Now().Unix(),
		Metadata:  metadata,
	})
}

func (a *LynxApp) listConfigurablePlugins(pm TypedPluginManager) []runtimeConfigPlugin {
	if pm == nil {
		return nil
	}
	manager, ok := pm.(*DefaultPluginManager[plugins.Plugin])
	if !ok {
		return nil
	}
	list := manager.listPluginsInternal()
	filtered := make([]runtimeConfigPlugin, 0, len(list))
	for _, p := range list {
		caps := plugins.DescribePluginCapabilities(p)
		configurable, ok := p.(plugins.Configurable)
		if !ok {
			continue
		}
		if caps.ProtocolExplicit && !caps.Protocol.ConfigHotReload {
			continue
		}
		item := runtimeConfigPlugin{
			Plugin:       p,
			Configurable: configurable,
			Caps:         caps,
		}
		if validator, ok := p.(plugins.ConfigValidator); ok {
			item.Validator = validator
		}
		if rollbacker, ok := p.(plugins.ConfigRollbacker); ok {
			item.Rollbacker = rollbacker
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func (a *LynxApp) validatePluginConfigurations(pluginList []runtimeConfigPlugin, cfg config.Config) error {
	for _, plugin := range pluginList {
		if plugin.Caps.ProtocolExplicit && plugin.Caps.Protocol.ConfigValidation && plugin.Validator == nil {
			return fmt.Errorf("plugin %s declares config validation support but does not implement ConfigValidator", plugin.Plugin.Name())
		}
		if plugin.Validator == nil {
			continue
		}
		if err := plugin.Validator.ValidateConfig(cfg); err != nil {
			return fmt.Errorf("plugin %s validation failed: %w", plugin.Plugin.Name(), err)
		}
	}
	return nil
}

func (a *LynxApp) applyPluginConfigurations(pluginList []runtimeConfigPlugin, cfg config.Config) ([]runtimeConfigPlugin, *configApplyError) {
	applied := make([]runtimeConfigPlugin, 0, len(pluginList))
	for _, plugin := range pluginList {
		if err := plugin.Configurable.Configure(cfg); err != nil {
			return applied, &configApplyError{Plugin: plugin.Plugin, Err: err}
		}
		applied = append(applied, plugin)
	}
	return applied, nil
}

func (a *LynxApp) rollbackPluginConfigurations(applied []runtimeConfigPlugin, oldCfg config.Config) error {
	var firstErr error
	for i := len(applied) - 1; i >= 0; i-- {
		plugin := applied[i]
		var err error
		if plugin.Configurable != nil {
			err = plugin.Configurable.Configure(oldCfg)
		} else if plugin.Rollbacker != nil {
			err = plugin.Rollbacker.RollbackConfig(oldCfg)
		}
		if err != nil && firstErr == nil {
			firstErr = fmt.Errorf("plugin %s rollback failed: %w", plugin.Plugin.Name(), err)
		}
	}
	return firstErr
}

// emitSystemEvent emits a system event to the unified event system
func (a *LynxApp) emitSystemEvent(eventType events.EventType, metadata map[string]any) {
	if a.pluginManager == nil {
		return
	}

	rt := a.pluginManager.GetRuntime()
	if rt == nil {
		return
	}

	// Create system event
	pluginEvent := plugins.PluginEvent{
		Type:      plugins.EventType(fmt.Sprintf("system.%d", eventType)),
		Priority:  plugins.PriorityHigh,
		Source:    "lynx-app",
		Category:  "system",
		PluginID:  "system",
		Status:    plugins.StatusActive,
		Timestamp: time.Now().Unix(),
		Metadata:  metadata,
	}

	// Emit through runtime
	rt.EmitEvent(pluginEvent)
}

// Close gracefully shuts down the Lynx application
func (a *LynxApp) Close() error {
	if a == nil {
		return nil
	}

	// Emit system shutdown event
	a.emitSystemEvent(events.EventSystemShutdown, map[string]any{
		"app_name":    a.name,
		"app_version": a.version,
		"host":        a.host,
		"reason":      "application_close",
	})

	// Close plugin manager (unload plugins) then shutdown runtime
	if a.pluginManager != nil {
		a.pluginManager.UnloadPlugins()
		if rt := a.pluginManager.GetRuntime(); rt != nil {
			rt.Shutdown()
		}
	}

	// Close gRPC connections to prevent resource leaks
	a.grpcSubsMu.Lock()
	grpcSubsCopy := make(map[string]*grpc.ClientConn)
	for k, v := range a.grpcSubs {
		grpcSubsCopy[k] = v
	}
	a.grpcSubs = nil
	a.grpcSubsMu.Unlock()

	// Close connections outside the lock to avoid holding lock during I/O
	for serviceName, conn := range grpcSubsCopy {
		if conn != nil {
			if err := conn.Close(); err != nil {
				log.Errorf("Failed to close gRPC connection for service %s: %v", serviceName, err)
			} else {
				log.Debugf("Successfully closed gRPC connection for service %s", serviceName)
			}
		}
	}

	// Stop health check before closing event bus
	if a.eventListenerManager != nil {
		a.eventListenerManager.Clear()
	}
	if a.eventManager != nil {
		a.eventManager.StopHealthCheck()
		if err := a.eventManager.Close(); err != nil {
			log.Errorf("Failed to close app event bus: %v", err)
		}
	}

	// Close global configuration to stop watcher goroutines and release resources.
	if a.globalConf != nil {
		if err := a.globalConf.Close(); err != nil {
			log.Errorf("Failed to close global configuration: %v", err)
		}
		a.globalConf = nil
	}

	// Clear global singleton instance if this app is the published default.
	clearDefaultAppIf(a)

	// Fix Bug 2: Cleanup memory stats cache goroutine to prevent goroutine leak
	// This ensures the background goroutine for memory stats updates is properly shut down
	cleanupMemoryStatsCache()

	// Reset initialization state
	// Note: sync.Once cannot be reset, so re-initialization after Close requires restart
	// This is acceptable as applications typically don't re-initialize after shutdown
	resetInitState()

	return nil
}

// Shutdown is an alias for Close for backward compatibility
func (a *LynxApp) Shutdown() error {
	return a.Close()
}

// GetResourceStats returns resource statistics from the plugin manager
func (a *LynxApp) GetResourceStats() map[string]any {
	if a == nil {
		return nil
	}
	if pm := a.GetPluginManager(); pm != nil {
		return pm.GetResourceStats()
	}
	return nil
}

// GetUnloadFailures returns plugin unload failures for monitoring
func (a *LynxApp) GetUnloadFailures() []UnloadFailureRecord {
	if a == nil {
		return nil
	}
	if pm := a.GetPluginManager(); pm != nil {
		return pm.GetUnloadFailures()
	}
	return nil
}

// ClearUnloadFailures clears recorded unload failures
func (a *LynxApp) ClearUnloadFailures() {
	if a == nil {
		return
	}
	if pm := a.GetPluginManager(); pm != nil {
		pm.ClearUnloadFailures()
	}
}

// LoadPlugins loads plugins through the app-owned plugin manager and then wires
// application-level subscriptions that depend on started plugins/control plane.
func (a *LynxApp) LoadPlugins() error {
	if a == nil {
		return fmt.Errorf("lynx app instance is nil")
	}
	pm := a.GetPluginManager()
	if pm == nil {
		return fmt.Errorf("plugin manager is nil")
	}
	if a.globalConf == nil {
		return fmt.Errorf("global configuration is nil")
	}

	if err := pm.LoadPlugins(a.globalConf); err != nil {
		return err
	}

	if err := a.configureGrpcSubscriptions(); err != nil {
		pm.UnloadPlugins()
		return err
	}

	return nil
}

// LoadPluginsByName loads a subset of plugins through the app-owned plugin manager.
func (a *LynxApp) LoadPluginsByName(names []string) error {
	if a == nil {
		return fmt.Errorf("lynx app instance is nil")
	}
	pm := a.GetPluginManager()
	if pm == nil {
		return fmt.Errorf("plugin manager is nil")
	}
	if a.globalConf == nil {
		return fmt.Errorf("global configuration is nil")
	}

	if err := pm.LoadPluginsByName(a.globalConf, names); err != nil {
		return err
	}

	if err := a.configureGrpcSubscriptions(); err != nil {
		pm.UnloadPluginsByName(names)
		return err
	}

	return nil
}

func (a *LynxApp) configureGrpcSubscriptions() error {
	if a == nil || a.bootConfig == nil || a.bootConfig.Lynx == nil || a.bootConfig.Lynx.Subscriptions == nil {
		a.replaceGrpcSubscriptions(nil)
		return nil
	}

	subs := a.bootConfig.Lynx.Subscriptions
	if len(subs.GetGrpc()) == 0 {
		a.replaceGrpcSubscriptions(nil)
		return nil
	}

	controlPlane := a.GetControlPlane()
	if controlPlane == nil {
		return fmt.Errorf("grpc subscriptions configured but control plane is not available (install a control plane plugin)")
	}

	disc := controlPlane.NewServiceDiscovery()
	if disc == nil {
		return fmt.Errorf("grpc subscriptions configured but service discovery is not available")
	}

	routerFactory := func(service string) selector.NodeFilter {
		return controlPlane.NewNodeRouter(service)
	}

	var tlsProviders *subscribe.ClientTLSProviders
	if hasTLSSubscription(subs) {
		tlsProviders = &subscribe.ClientTLSProviders{
			ConfigProvider: controlPlane.GetConfig,
			DefaultRootCA:  a.defaultRootCAProvider(),
		}
	}

	conns, err := subscribe.BuildGrpcSubscriptions(subs, disc, routerFactory, tlsProviders)
	if err != nil {
		closeGrpcConnections(conns)
		return fmt.Errorf("build grpc subscriptions failed: %w", err)
	}

	a.replaceGrpcSubscriptions(conns)
	return nil
}

func hasTLSSubscription(subs *conf.Subscriptions) bool {
	if subs == nil || len(subs.GetGrpc()) == 0 {
		return false
	}
	for _, g := range subs.GetGrpc() {
		if g.GetTls() {
			return true
		}
	}
	return false
}

func (a *LynxApp) defaultRootCAProvider() func() []byte {
	return func() []byte {
		if a == nil || a.Certificate() == nil {
			return nil
		}
		return a.Certificate().GetRootCACertificate()
	}
}

func (a *LynxApp) replaceGrpcSubscriptions(conns map[string]*grpc.ClientConn) {
	if a == nil {
		return
	}

	next := make(map[string]*grpc.ClientConn, len(conns))
	for name, conn := range conns {
		next[name] = conn
	}

	a.grpcSubsMu.Lock()
	prev := a.grpcSubs
	a.grpcSubs = next
	a.grpcSubsMu.Unlock()

	for name, oldConn := range prev {
		newConn, stillPresent := next[name]
		if stillPresent && newConn == oldConn {
			continue
		}
		if oldConn != nil {
			if err := oldConn.Close(); err != nil {
				log.Errorf("Failed to close previous gRPC connection for service %s: %v", name, err)
			}
		}
	}
}

func closeGrpcConnections(conns map[string]*grpc.ClientConn) {
	for name, conn := range conns {
		if conn == nil {
			continue
		}
		if err := conn.Close(); err != nil {
			log.Errorf("Failed to close gRPC connection for service %s: %v", name, err)
		}
	}
}

func (a *LynxApp) injectRuntimeEventAdapter() {
	if a == nil || a.eventAdapter == nil {
		return
	}
	pm := a.GetPluginManager()
	if pm == nil {
		return
	}
	rt := pm.GetRuntime()
	if rt == nil {
		return
	}
	type eventAdapterSetter interface {
		SetEventBusAdapter(plugins.EventBusAdapter)
	}
	if setter, ok := rt.(eventAdapterSetter); ok {
		setter.SetEventBusAdapter(a.eventAdapter)
	}
}

// EventManager returns the app-owned event bus manager.
func (a *LynxApp) EventManager() *events.EventBusManager {
	if a == nil {
		return nil
	}
	return a.eventManager
}

// EventListenerManager returns the app-owned listener manager.
func (a *LynxApp) EventListenerManager() *events.EventListenerManager {
	if a == nil {
		return nil
	}
	return a.eventListenerManager
}

// EventAdapter returns the app-owned plugin event adapter.
func (a *LynxApp) EventAdapter() plugins.EventBusAdapter {
	if a == nil {
		return nil
	}
	return a.eventAdapter
}

// EventSystemHealth returns the health snapshot of the app-owned event system.
func (a *LynxApp) EventSystemHealth() *events.EventSystemHealth {
	if a == nil || a.eventManager == nil {
		return nil
	}
	return a.eventManager.GetEventSystemHealth()
}

// EventMetrics returns the app-owned event monitor metrics.
func (a *LynxApp) EventMetrics() map[string]interface{} {
	if a == nil || a.eventManager == nil || a.eventManager.GetMonitor() == nil {
		return nil
	}
	return a.eventManager.GetMonitor().GetMetrics()
}

// ConfigReloadPlan returns the current plugin manager's config reload classification.
func (a *LynxApp) ConfigReloadPlan() ConfigReloadPlan {
	if a == nil || a.pluginManager == nil {
		return ConfigReloadPlan{}
	}
	return a.pluginManager.GetConfigReloadPlan()
}
