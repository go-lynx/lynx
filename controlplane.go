// Package lynx provides the core application framework for building microservices.
//
// This file (controlplane.go) contains the control plane interfaces and implementation:
//   - ControlPlane: Main interface for service management (composed of smaller interfaces)
//   - RateLimiter: HTTP and gRPC rate limiting
//   - ServiceRegistry: Service registration and discovery
//   - RouteManager: Service routing and node filtering
//   - ConfigManager: Configuration source management
//   - SystemCore: Basic application information
//   - DefaultControlPlane: Basic implementation for local development
//
// Plugins may implement the full ControlPlane interface (e.g. Polaris, Nacos) or
// register individual capabilities via SetRateLimiter, SetServiceRegistry, etc.
package lynx

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-lynx/lynx/log"
	"github.com/go-lynx/lynx/plugins"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/selector"
)

// ControlPlane defines the interface for managing core application services
// including rate limiting, service discovery, routing, and configuration
type ControlPlane interface {
	// SystemCore provides basic application information.
	SystemCore
	// RateLimiter provides rate limiting functionality for HTTP and gRPC services.
	RateLimiter
	// ServiceRegistry provides service registration and discovery functionality.
	ServiceRegistry
	// RouteManager provides service routing functionality.
	RouteManager
	// ConfigManager provides configuration management functionality.
	ConfigManager
}

// SystemCore provides basic application information
type SystemCore interface {
	// GetNamespace returns the current application control plane namespace
	GetNamespace() string
}

// RateLimiter provides rate limiting functionality for HTTP and gRPC services
type RateLimiter interface {
	// HTTPRateLimit returns middleware for HTTP rate limiting
	HTTPRateLimit() middleware.Middleware
	// GRPCRateLimit returns middleware for gRPC rate limiting
	GRPCRateLimit() middleware.Middleware
}

// ServiceRegistry provides service registration and discovery functionality
type ServiceRegistry interface {
	// NewServiceRegistry creates a new service registrar
	NewServiceRegistry() registry.Registrar
	// NewServiceDiscovery creates a new service discovery client
	NewServiceDiscovery() registry.Discovery
}

// RouteManager provides service routing functionality
type RouteManager interface {
	// NewNodeRouter creates a new node filter for the specified service
	NewNodeRouter(serviceName string) selector.NodeFilter
}

// ConfigManager provides configuration management functionality
type ConfigManager interface {
	// GetConfig retrieves configuration from a source using the specified file and group
	GetConfig(fileName string, group string) (config.Source, error)
}

// MultiConfigControlPlane extends ControlPlane to support multiple configuration sources
type MultiConfigControlPlane interface {
	ControlPlane
	// GetConfigSources retrieves all configuration sources for multi-config loading
	GetConfigSources() ([]config.Source, error)
}

// ControlPlaneCapability describes a single provider-facing capability exposed by a control plane.
type ControlPlaneCapability string

const (
	ControlPlaneCapabilityConfig            ControlPlaneCapability = "config"
	ControlPlaneCapabilityRegistry          ControlPlaneCapability = "registry"
	ControlPlaneCapabilityDiscovery         ControlPlaneCapability = "discovery"
	ControlPlaneCapabilityRouter            ControlPlaneCapability = "router"
	ControlPlaneCapabilityRateLimit         ControlPlaneCapability = "rate_limit"
	ControlPlaneCapabilityTrafficProtection ControlPlaneCapability = "traffic_protection"
	ControlPlaneCapabilityWatcher           ControlPlaneCapability = "watcher"
)

// ControlPlaneCapabilityReporter allows providers to declare an explicit capability set.
type ControlPlaneCapabilityReporter interface {
	ControlPlaneCapabilities() []ControlPlaneCapability
}

// ControlPlaneConfigTarget describes a remote config item that belongs to the control plane.
type ControlPlaneConfigTarget struct {
	FileName      string
	Group         string
	Priority      int
	MergeStrategy string
}

// Key returns a stable identifier for a config target.
func (t ControlPlaneConfigTarget) Key() string {
	return fmt.Sprintf("%s|%s|%d|%s", t.FileName, t.Group, t.Priority, t.MergeStrategy)
}

// ControlPlaneConfigWatcherProvider exposes watcher-based remote config backfill hooks.
type ControlPlaneConfigWatcherProvider interface {
	GetConfigWatchTargets(appName string) ([]ControlPlaneConfigTarget, error)
	WatchControlPlaneConfig(ctx context.Context, target ControlPlaneConfigTarget) (config.Watcher, error)
}

type controlPlaneBackfillState struct {
	cancel context.CancelFunc
}

var controlPlaneBackfills sync.Map

// ControlPlaneCapabilityResourceName returns the shared runtime resource name for a capability alias.
func ControlPlaneCapabilityResourceName(provider string, capability ControlPlaneCapability) string {
	return fmt.Sprintf("%s.%s", provider, capability)
}

// ControlPlaneCapabilitiesOf returns the explicit plus inferred capability set of a control plane.
func ControlPlaneCapabilitiesOf(plane any) []ControlPlaneCapability {
	if plane == nil {
		return nil
	}

	seen := make(map[ControlPlaneCapability]struct{}, 8)
	caps := make([]ControlPlaneCapability, 0, 8)
	add := func(capability ControlPlaneCapability) {
		if capability == "" {
			return
		}
		if _, exists := seen[capability]; exists {
			return
		}
		seen[capability] = struct{}{}
		caps = append(caps, capability)
	}

	if reporter, ok := plane.(ControlPlaneCapabilityReporter); ok {
		for _, capability := range reporter.ControlPlaneCapabilities() {
			add(capability)
		}
	}
	if _, ok := plane.(ConfigManager); ok {
		add(ControlPlaneCapabilityConfig)
	}
	if _, ok := plane.(ServiceRegistry); ok {
		add(ControlPlaneCapabilityRegistry)
		add(ControlPlaneCapabilityDiscovery)
	}
	if _, ok := plane.(RouteManager); ok {
		add(ControlPlaneCapabilityRouter)
	}
	if _, ok := plane.(RateLimiter); ok {
		add(ControlPlaneCapabilityRateLimit)
	}
	if _, ok := plane.(ControlPlaneConfigWatcherProvider); ok {
		add(ControlPlaneCapabilityWatcher)
	}

	return caps
}

// StartControlPlaneWatcher normalizes different watcher-start semantics under one contract.
func StartControlPlaneWatcher(ctx context.Context, watcher config.Watcher) error {
	if watcher == nil {
		return fmt.Errorf("control plane watcher is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	switch starter := any(watcher).(type) {
	case interface{ Start(context.Context) error }:
		if err := starter.Start(ctx); err != nil {
			return err
		}
	case interface{ Start() }:
		starter.Start()
	}

	go func() {
		<-ctx.Done()
		_ = watcher.Stop()
	}()

	return nil
}

// RegisterControlPlaneCapabilityResources publishes standardized shared resource aliases for a provider.
func RegisterControlPlaneCapabilityResources(rt plugins.Runtime, provider string, plane any) error {
	if rt == nil || provider == "" || plane == nil {
		return nil
	}
	if err := rt.RegisterSharedResource(provider, plane); err != nil {
		return err
	}

	aliases := map[ControlPlaneCapability]any{
		ControlPlaneCapabilityConfig:    plane,
		ControlPlaneCapabilityRegistry:  plane,
		ControlPlaneCapabilityDiscovery: plane,
		ControlPlaneCapabilityRouter:    plane,
		ControlPlaneCapabilityRateLimit: plane,
		ControlPlaneCapabilityWatcher:   plane,
	}
	if cm, ok := plane.(ConfigManager); ok {
		aliases[ControlPlaneCapabilityConfig] = cm
	}
	if sr, ok := plane.(ServiceRegistry); ok {
		aliases[ControlPlaneCapabilityRegistry] = sr
		aliases[ControlPlaneCapabilityDiscovery] = sr
	}
	if rm, ok := plane.(RouteManager); ok {
		aliases[ControlPlaneCapabilityRouter] = rm
	}
	if rl, ok := plane.(RateLimiter); ok {
		aliases[ControlPlaneCapabilityRateLimit] = rl
	}
	if watcherProvider, ok := plane.(ControlPlaneConfigWatcherProvider); ok {
		aliases[ControlPlaneCapabilityWatcher] = watcherProvider
	}

	for _, capability := range ControlPlaneCapabilitiesOf(plane) {
		alias := aliases[capability]
		if alias == nil {
			alias = plane
		}
		if err := rt.RegisterSharedResource(ControlPlaneCapabilityResourceName(provider, capability), alias); err != nil {
			return err
		}
	}

	return nil
}

// DefaultControlPlane provides a basic implementation of the ControlPlane interface
// for local development and testing purposes
type DefaultControlPlane struct {
}

// HTTPRateLimit implements the RateLimiter interface for HTTP rate limiting
func (c *DefaultControlPlane) HTTPRateLimit() middleware.Middleware {
	return nil
}

// GRPCRateLimit implements the RateLimiter interface for gRPC rate limiting
func (c *DefaultControlPlane) GRPCRateLimit() middleware.Middleware {
	return nil
}

// NewServiceRegistry implements the ServiceRegistry interface for service registration
func (c *DefaultControlPlane) NewServiceRegistry() registry.Registrar {
	return nil
}

// NewServiceDiscovery implements the ServiceRegistry interface for service discovery
func (c *DefaultControlPlane) NewServiceDiscovery() registry.Discovery {
	return nil
}

// NewNodeRouter implements the RouteManager interface for service routing
func (c *DefaultControlPlane) NewNodeRouter(serviceName string) selector.NodeFilter {
	return nil
}

// GetConfig implements the ConfigManager interface for configuration management
func (c *DefaultControlPlane) GetConfig(fileName string, group string) (config.Source, error) {
	return nil, nil
}

// GetNamespace implements the SystemCore interface for namespace management
func (c *DefaultControlPlane) GetNamespace() string {
	return ""
}

// compositeControlPlane merges a full ControlPlane with optional capability overrides.
// Individual capabilities take precedence when set; otherwise delegates to the full control plane.
// Immutable snapshot created per GetControlPlane() call.
type compositeControlPlane struct {
	controlPlane    ControlPlane
	rateLimiter     RateLimiter
	serviceRegistry ServiceRegistry
	routeManager    RouteManager
	configManager   ConfigManager
	systemCore      SystemCore
}

// Compile-time check: compositeControlPlane implements ControlPlane and MultiConfigControlPlane
var (
	_ ControlPlane            = (*compositeControlPlane)(nil)
	_ MultiConfigControlPlane = (*compositeControlPlane)(nil)
)

func (c *compositeControlPlane) HTTPRateLimit() middleware.Middleware {
	if c.rateLimiter != nil {
		return c.rateLimiter.HTTPRateLimit()
	}
	if c.controlPlane != nil {
		return c.controlPlane.HTTPRateLimit()
	}
	return nil
}

func (c *compositeControlPlane) GRPCRateLimit() middleware.Middleware {
	if c.rateLimiter != nil {
		return c.rateLimiter.GRPCRateLimit()
	}
	if c.controlPlane != nil {
		return c.controlPlane.GRPCRateLimit()
	}
	return nil
}

func (c *compositeControlPlane) NewServiceRegistry() registry.Registrar {
	if c.serviceRegistry != nil {
		return c.serviceRegistry.NewServiceRegistry()
	}
	if c.controlPlane != nil {
		return c.controlPlane.NewServiceRegistry()
	}
	return nil
}

func (c *compositeControlPlane) NewServiceDiscovery() registry.Discovery {
	if c.serviceRegistry != nil {
		return c.serviceRegistry.NewServiceDiscovery()
	}
	if c.controlPlane != nil {
		return c.controlPlane.NewServiceDiscovery()
	}
	return nil
}

func (c *compositeControlPlane) NewNodeRouter(serviceName string) selector.NodeFilter {
	if c.routeManager != nil {
		return c.routeManager.NewNodeRouter(serviceName)
	}
	if c.controlPlane != nil {
		return c.controlPlane.NewNodeRouter(serviceName)
	}
	return nil
}

func (c *compositeControlPlane) GetConfig(fileName string, group string) (config.Source, error) {
	if c.configManager != nil {
		return c.configManager.GetConfig(fileName, group)
	}
	if c.controlPlane != nil {
		return c.controlPlane.GetConfig(fileName, group)
	}
	return nil, nil
}

func (c *compositeControlPlane) GetNamespace() string {
	if c.systemCore != nil {
		return c.systemCore.GetNamespace()
	}
	if c.controlPlane != nil {
		return c.controlPlane.GetNamespace()
	}
	return ""
}

func (c *compositeControlPlane) GetConfigSources() ([]config.Source, error) {
	if c.controlPlane != nil {
		if multi, ok := c.controlPlane.(MultiConfigControlPlane); ok {
			return multi.GetConfigSources()
		}
	}
	return nil, fmt.Errorf("control plane does not support multi-config")
}

// GetControlPlane returns the current control plane instance (composite of all capabilities).
// Returns nil when no control plane and no capabilities are set.
// Plugins implementing full ControlPlane or individual capabilities are merged.
func (a *LynxApp) GetControlPlane() ControlPlane {
	if a == nil {
		return nil
	}
	return a.getCompositeControlPlane()
}

// getCompositeControlPlane returns the composite control plane for this app, or nil if empty.
// The composite delegates to individual capabilities first, then to the full control plane.
func (a *LynxApp) getCompositeControlPlane() ControlPlane {
	a.controlPlaneMu.RLock()
	cp := a.controlPlane
	rl := a.rateLimiter
	sr := a.serviceRegistry
	rm := a.routeManager
	cm := a.configManager
	sc := a.systemCore
	a.controlPlaneMu.RUnlock()
	// Return nil when no capabilities are set (matches legacy behavior)
	if cp == nil && rl == nil && sr == nil && rm == nil && cm == nil && sc == nil {
		return nil
	}
	return &compositeControlPlane{
		controlPlane:    cp,
		rateLimiter:     rl,
		serviceRegistry: sr,
		routeManager:    rm,
		configManager:   cm,
		systemCore:      sc,
	}
}

// SetControlPlane sets the control plane instance for the application.
// When plane implements the full ControlPlane interface, all capabilities are extracted.
// Existing plugins (Polaris, Nacos) continue to work unchanged.
func (a *LynxApp) SetControlPlane(plane ControlPlane) error {
	if a == nil {
		return fmt.Errorf("lynx app instance is nil")
	}
	if plane == nil {
		return fmt.Errorf("control plane instance cannot be nil")
	}
	a.controlPlaneMu.Lock()
	defer a.controlPlaneMu.Unlock()
	a.controlPlane = plane
	// Extract capabilities from full ControlPlane for composite delegation
	if rl, ok := plane.(RateLimiter); ok {
		a.rateLimiter = rl
	}
	if sr, ok := plane.(ServiceRegistry); ok {
		a.serviceRegistry = sr
	}
	if rm, ok := plane.(RouteManager); ok {
		a.routeManager = rm
	}
	if cm, ok := plane.(ConfigManager); ok {
		a.configManager = cm
	}
	if sc, ok := plane.(SystemCore); ok {
		a.systemCore = sc
	}
	a.stopControlPlaneConfigBackfill()
	return nil
}

// SetRateLimiter sets the rate limiter capability (for partial implementations)
func (a *LynxApp) SetRateLimiter(r RateLimiter) error {
	if a == nil {
		return fmt.Errorf("lynx app instance is nil")
	}
	a.controlPlaneMu.Lock()
	defer a.controlPlaneMu.Unlock()
	a.rateLimiter = r
	return nil
}

// SetServiceRegistry sets the service registry capability (for partial implementations)
func (a *LynxApp) SetServiceRegistry(sr ServiceRegistry) error {
	if a == nil {
		return fmt.Errorf("lynx app instance is nil")
	}
	a.controlPlaneMu.Lock()
	defer a.controlPlaneMu.Unlock()
	a.serviceRegistry = sr
	return nil
}

// SetRouteManager sets the route manager capability (for partial implementations)
func (a *LynxApp) SetRouteManager(rm RouteManager) error {
	if a == nil {
		return fmt.Errorf("lynx app instance is nil")
	}
	a.controlPlaneMu.Lock()
	defer a.controlPlaneMu.Unlock()
	a.routeManager = rm
	return nil
}

// SetConfigManager sets the config manager capability (for partial implementations)
func (a *LynxApp) SetConfigManager(cm ConfigManager) error {
	if a == nil {
		return fmt.Errorf("lynx app instance is nil")
	}
	a.controlPlaneMu.Lock()
	defer a.controlPlaneMu.Unlock()
	a.configManager = cm
	return nil
}

// SetSystemCore sets the system core capability (for partial implementations)
func (a *LynxApp) SetSystemCore(sc SystemCore) error {
	if a == nil {
		return fmt.Errorf("lynx app instance is nil")
	}
	a.controlPlaneMu.Lock()
	defer a.controlPlaneMu.Unlock()
	a.systemCore = sc
	return nil
}

// InitControlPlaneConfig initializes the control plane configuration
// It loads the configuration from the specified source and sets up the global configuration
func (a *LynxApp) InitControlPlaneConfig() (config.Config, error) {
	if a == nil {
		return nil, fmt.Errorf("lynx app instance is nil")
	}

	// Create new configuration if control plane is not initialized
	if a.GetControlPlane() == nil {
		cfg := config.New()
		if cfg == nil {
			return nil, fmt.Errorf("failed to create new configuration")
		}
		return cfg, nil
	}

	// Get configuration sources from control plane
	configSources, err := a.GetControlPlaneConfigSources()
	if err != nil {
		return nil, fmt.Errorf("failed to get configuration sources: %w", err)
	}

	// Create and load configuration with multiple sources
	cfg := config.New(config.WithSource(configSources...))
	if cfg == nil {
		return nil, fmt.Errorf("failed to create configuration with sources")
	}

	// Load configuration
	if err := cfg.Load(); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Set global configuration
	if err := a.SetGlobalConfig(cfg); err != nil {
		return nil, fmt.Errorf("failed to set global configuration: %w", err)
	}
	if err := a.startControlPlaneConfigBackfill(cfg); err != nil {
		return nil, fmt.Errorf("failed to start control plane config backfill: %w", err)
	}

	return cfg, nil
}

// GetControlPlaneConfigSources gets all configuration sources from the control plane
// This method supports loading multiple configuration files from remote sources
func (a *LynxApp) GetControlPlaneConfigSources() ([]config.Source, error) {
	if a == nil || a.GetControlPlane() == nil {
		return nil, fmt.Errorf("control plane not available")
	}

	// Check if control plane supports multi-config loading
	if multiConfigPlane, ok := a.GetControlPlane().(MultiConfigControlPlane); ok {
		return multiConfigPlane.GetConfigSources()
	}

	// Fallback to single config loading for backward compatibility
	configFileName := fmt.Sprintf("%s.yaml", a.name)
	namespace := a.GetControlPlane().GetNamespace()

	// Log configuration loading attempt
	log.Infof("Loading configuration - File: [%s] Group: [%s] Namespace: [%s]",
		configFileName, a.name, namespace)

	// Get configuration source from control plane
	configSource, err := a.GetControlPlane().GetConfig(configFileName, a.name)
	if err != nil {
		log.Errorf("Failed to load configuration - File: [%s] Group: [%s] Namespace: [%s] Error: %v",
			configFileName, a.name, namespace, err)
		return nil, fmt.Errorf("failed to load configuration source: %w", err)
	}

	return []config.Source{configSource}, nil
}

// GetServiceRegistry returns a new service registry instance for this app.
func (a *LynxApp) GetServiceRegistry() (registry.Registrar, error) {
	if a == nil || a.GetControlPlane() == nil {
		// No control plane available, return nil registrar (no service registration)
		return nil, nil
	}
	reg := a.GetControlPlane().NewServiceRegistry()
	// Return the registrar even if it's nil (no service registration)
	return reg, nil
}

// GetServiceDiscovery returns a new service discovery instance for this app.
// This app-level lookup is the compatibility fallback for callers that do not
// (or cannot yet) rely on unified runtime resource aliases. Transport/client
// integrations should remain functional as long as the default app exposes a
// discovery capability, even if provider-specific alias wiring is still being
// converged.
func (a *LynxApp) GetServiceDiscovery() (registry.Discovery, error) {
	if a == nil || a.GetControlPlane() == nil {
		// No control plane available, return nil discovery (no service discovery)
		return nil, nil
	}
	disc := a.GetControlPlane().NewServiceDiscovery()
	// Return the discovery even if it's nil (no service discovery)
	return disc, nil
}

// GetControlPlaneCapabilities returns the merged capability set currently exposed by the app.
func (a *LynxApp) GetControlPlaneCapabilities() []ControlPlaneCapability {
	if a == nil {
		return nil
	}
	return ControlPlaneCapabilitiesOf(a.GetControlPlane())
}

func (a *LynxApp) stopControlPlaneConfigBackfill() {
	if a == nil {
		return
	}
	if stateValue, ok := controlPlaneBackfills.LoadAndDelete(a); ok {
		if state, ok := stateValue.(*controlPlaneBackfillState); ok && state != nil && state.cancel != nil {
			state.cancel()
		}
	}
}

func (a *LynxApp) startControlPlaneConfigBackfill(cfg config.Config) error {
	if a == nil || cfg == nil {
		return nil
	}

	provider, ok := a.GetControlPlane().(ControlPlaneConfigWatcherProvider)
	if !ok || provider == nil {
		a.stopControlPlaneConfigBackfill()
		return nil
	}

	targets, err := provider.GetConfigWatchTargets(a.name)
	if err != nil {
		return err
	}

	a.stopControlPlaneConfigBackfill()
	if len(targets) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	controlPlaneBackfills.Store(a, &controlPlaneBackfillState{cancel: cancel})

	for _, target := range targets {
		target := target
		go a.consumeControlPlaneConfigTarget(ctx, cfg, provider, target)
	}

	return nil
}

func (a *LynxApp) consumeControlPlaneConfigTarget(
	ctx context.Context,
	cfg config.Config,
	provider ControlPlaneConfigWatcherProvider,
	target ControlPlaneConfigTarget,
) {
	for {
		if ctx.Err() != nil {
			return
		}

		watcher, err := provider.WatchControlPlaneConfig(ctx, target)
		if err != nil {
			log.Warnf("failed to start control plane config watcher for %s: %v", target.Key(), err)
			if waitControlPlaneWatchRetry(ctx, 2*time.Second) {
				return
			}
			continue
		}
		if watcher == nil {
			log.Warnf("control plane config watcher for %s is nil", target.Key())
			return
		}

		err = a.runControlPlaneWatcherLoop(ctx, cfg, watcher, target)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Warnf("control plane config watcher for %s stopped, retrying: %v", target.Key(), err)
		}
		if waitControlPlaneWatchRetry(ctx, 2*time.Second) {
			return
		}
	}
}

func (a *LynxApp) runControlPlaneWatcherLoop(
	ctx context.Context,
	cfg config.Config,
	watcher config.Watcher,
	target ControlPlaneConfigTarget,
) error {
	defer func() {
		_ = watcher.Stop()
	}()

	for {
		kvs, err := watcher.Next()
		if err != nil {
			return err
		}
		if len(kvs) == 0 {
			continue
		}
		if err := cfg.Load(); err != nil {
			log.Warnf("failed to reload config snapshot after control plane update %s: %v", target.Key(), err)
			continue
		}

		managedPlugins := 0
		if pm := a.GetPluginManager(); pm != nil {
			managedPlugins = len(Plugins(pm))
		}

		if managedPlugins > 0 {
			log.Warnf(
				"control plane config updated for %s; global config snapshot reloaded in place, but %d managed plugins may still require restart to consume the new values",
				target.Key(),
				managedPlugins,
			)
			continue
		}

		log.Infof("control plane config updated for %s and reloaded into the global config snapshot", target.Key())
	}
}

func waitControlPlaneWatchRetry(ctx context.Context, delay time.Duration) bool {
	select {
	case <-ctx.Done():
		return true
	case <-time.After(delay):
		return false
	}
}
