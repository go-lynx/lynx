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
	"fmt"

	"github.com/go-lynx/lynx/log"

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
	configFileName := fmt.Sprintf("%s.yaml", GetName())
	namespace := a.GetControlPlane().GetNamespace()

	// Log configuration loading attempt
	log.Infof("Loading configuration - File: [%s] Group: [%s] Namespace: [%s]",
		configFileName, GetName(), namespace)

	// Get configuration source from control plane
	configSource, err := a.GetControlPlane().GetConfig(configFileName, GetName())
	if err != nil {
		log.Errorf("Failed to load configuration - File: [%s] Group: [%s] Namespace: [%s] Error: %v",
			configFileName, GetName(), namespace, err)
		return nil, fmt.Errorf("failed to load configuration source: %w", err)
	}

	return []config.Source{configSource}, nil
}

// GetServiceRegistry returns a new service registry instance
func GetServiceRegistry() (registry.Registrar, error) {
	if Lynx() == nil || Lynx().GetControlPlane() == nil {
		// No control plane available, return nil registrar (no service registration)
		return nil, nil
	}
	reg := Lynx().GetControlPlane().NewServiceRegistry()
	// Return the registrar even if it's nil (no service registration)
	return reg, nil
}

// GetServiceDiscovery returns a new service discovery instance
func GetServiceDiscovery() (registry.Discovery, error) {
	if Lynx() == nil || Lynx().GetControlPlane() == nil {
		// No control plane available, return nil discovery (no service discovery)
		return nil, nil
	}
	disc := Lynx().GetControlPlane().NewServiceDiscovery()
	// Return the discovery even if it's nil (no service discovery)
	return disc, nil
}
