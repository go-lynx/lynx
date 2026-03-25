// Package lynx provides the core application framework for building microservices.
//
// This file (app.go) contains the LynxApp structure and main API entry points.
package lynx

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-lynx/lynx/conf"
	"github.com/go-lynx/lynx/events"
	"github.com/go-lynx/lynx/log"

	"github.com/go-kratos/kratos/v2/config"
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
	if app == nil {
		events.ClearDefaultEventBusProvider()
		events.ClearDefaultListenerManagerProvider()
		return
	}
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
	events.ClearDefaultEventBusProvider()
	events.ClearDefaultListenerManagerProvider()
	return true
}

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

	// controlPlane manages the application's external control-plane integration.
	// This is shell-facing composition, not the heart of plugin orchestration.
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

	// App-owned event facilities. Core runtime paths should prefer these over global singletons.
	eventManager         *events.EventBusManager
	eventListenerManager *events.EventListenerManager
	eventAdapter         plugins.EventBusAdapter
}

// Lynx returns the process-wide default LynxApp instance.
//
// Deprecated: prefer passing an explicit *LynxApp and using instance helpers
// such as GetTypedPluginFromApp, GetEventManagerFromApp, and
// GetEventListenerManagerFromApp.
func Lynx() *LynxApp {
	lynxMu.RLock()
	defer lynxMu.RUnlock()
	return lynxApp
}

// GetEventManagerFromApp returns the event bus manager owned by app.
func GetEventManagerFromApp(app *LynxApp) *events.EventBusManager {
	if app == nil {
		return nil
	}
	return app.EventManager()
}

// GetEventListenerManagerFromApp returns the listener manager owned by app.
func GetEventListenerManagerFromApp(app *LynxApp) *events.EventListenerManager {
	if app == nil {
		return nil
	}
	return app.EventListenerManager()
}

// GetEventAdapterFromApp returns the plugin event adapter owned by app.
func GetEventAdapterFromApp(app *LynxApp) plugins.EventBusAdapter {
	if app == nil {
		return nil
	}
	return app.EventAdapter()
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

// GetPluginManagerFromApp returns the plugin manager owned by app.
func GetPluginManagerFromApp(app *LynxApp) TypedPluginManager {
	if app == nil {
		return nil
	}
	return app.GetPluginManager()
}

// GetGlobalConfigFromApp returns the config owned by app.
func GetGlobalConfigFromApp(app *LynxApp) config.Config {
	if app == nil {
		return nil
	}
	return app.GetGlobalConfig()
}

// GetTypedPluginFromApp retrieves a typed plugin from an explicit app instance.
func GetTypedPluginFromApp[T plugins.Plugin](app *LynxApp, name string) (T, error) {
	var zero T
	if app == nil {
		return zero, fmt.Errorf("lynx application not initialized")
	}

	manager := app.GetTypedPluginManager()
	if manager == nil {
		return zero, fmt.Errorf("typed plugin manager not initialized")
	}

	return GetTypedPluginFromManager[T](manager, name)
}

// MustGetTypedPluginFromApp retrieves a typed plugin from an explicit app instance or panics.
func MustGetTypedPluginFromApp[T plugins.Plugin](app *LynxApp, name string) T {
	p, err := GetTypedPluginFromApp[T](app, name)
	if err != nil {
		panic(err)
	}
	return p
}

// GetTypedPlugin globally retrieves a type-safe plugin instance.
// Deprecated: prefer GetTypedPluginFromApp or GetTypedPluginFromManager to avoid relying on the process-wide default app.
func GetTypedPlugin[T plugins.Plugin](name string) (T, error) {
	return GetTypedPluginFromApp[T](Lynx(), name)
}

// SetGlobalConfig replaces the application configuration reference.
// Lynx core does not orchestrate runtime plugin reconfiguration; when plugins are
// already loaded, configuration changes should be applied via process restart or
// external rollout tools such as Kubernetes.
func (a *LynxApp) SetGlobalConfig(cfg config.Config) error {
	if a == nil {
		return fmt.Errorf("application instance is nil")
	}
	if cfg == nil {
		return fmt.Errorf("new configuration cannot be nil")
	}

	pm := a.GetPluginManager()
	oldCfg := a.globalConf

	if pm != nil {
		loaded := Plugins(pm)
		if len(loaded) > 0 {
			loadedNames := make([]string, 0, len(loaded))
			for _, plugin := range loaded {
				if plugin == nil {
					continue
				}
				loadedNames = append(loadedNames, plugin.Name())
			}
			if cfg != nil {
				_ = cfg.Close()
			}
			return fmt.Errorf("runtime configuration reload is not supported by lynx core; restart application to apply changes for plugins: %v", loadedNames)
		}
		pm.SetConfig(cfg)
		if rt := pm.GetRuntime(); rt != nil {
			rt.SetConfig(cfg)
		}
	}

	a.globalConf = cfg

	if oldCfg != nil {
		if err := oldCfg.Close(); err != nil {
			log.Errorf("Failed to close existing configuration: %v", err)
			return err
		}
	}

	return nil
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

// RestartRequirementReport returns the current plugin manager's restart-based
// compatibility report for configuration changes.
func (a *LynxApp) RestartRequirementReport() RestartRequirementReport {
	if a == nil || a.pluginManager == nil {
		return RestartRequirementReport{}
	}
	return a.pluginManager.GetRestartRequirementReport()
}

// ConfigReloadPlan returns the current plugin manager's compatibility view for
// older callers.
//
// Deprecated: prefer RestartRequirementReport(), which matches Lynx core's
// restart-based configuration model.
func (a *LynxApp) ConfigReloadPlan() ConfigReloadPlan {
	if a == nil || a.pluginManager == nil {
		return ConfigReloadPlan{}
	}
	return a.pluginManager.GetConfigReloadPlan()
}

type PluginRuntimeReport struct {
	ID           string
	Name         string
	Version      string
	Status       plugins.PluginStatus
	Capabilities plugins.PluginCapabilities
}

type RuntimeReport struct {
	AppName    string
	AppVersion string
	Host       string
	// RestartRequirementReport is the core-facing view for config-change handling.
	RestartRequirementReport RestartRequirementReport
	// ConfigReloadPlan is retained only for compatibility with older callers.
	ConfigReloadPlan ConfigReloadPlan
	Plugins          []PluginRuntimeReport
}

func (a *LynxApp) RuntimeReport() RuntimeReport {
	report := RuntimeReport{}
	if a == nil {
		return report
	}

	report.AppName = a.Name()
	report.AppVersion = a.Version()
	report.Host = a.Host()
	report.RestartRequirementReport = a.RestartRequirementReport()
	report.ConfigReloadPlan = a.ConfigReloadPlan()

	if a.pluginManager == nil {
		return report
	}

	list := Plugins(a.pluginManager)
	report.Plugins = make([]PluginRuntimeReport, 0, len(list))
	for _, p := range list {
		if p == nil {
			continue
		}
		report.Plugins = append(report.Plugins, PluginRuntimeReport{
			ID:           p.ID(),
			Name:         p.Name(),
			Version:      p.Version(),
			Status:       p.Status(p),
			Capabilities: plugins.DescribePluginCapabilities(p),
		})
	}

	return report
}
