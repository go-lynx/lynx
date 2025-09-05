// Package app provides core application functionality for the Lynx framework
package app

import (
	"fmt"

	"github.com/go-lynx/lynx/app/log"

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

// GetControlPlane returns the current control plane instance
func (a *LynxApp) GetControlPlane() ControlPlane {
	if a == nil {
		return nil
	}
	return Lynx().controlPlane
}

// SetControlPlane sets the control plane instance for the application
func (a *LynxApp) SetControlPlane(plane ControlPlane) error {
	if a == nil {
		return fmt.Errorf("lynx app instance is nil")
	}
	if plane == nil {
		return fmt.Errorf("control plane instance cannot be nil")
	}
	Lynx().controlPlane = plane
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
