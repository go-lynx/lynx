// Package app provides core application functionality for the Lynx framework
package app

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/selector"
)

// ControlPlane defines the interface for managing core application services
// including rate limiting, service discovery, routing, and configuration
type ControlPlane interface {
	Base
	Limiter
	Registry
	Router
	Config
}

// Base provides basic application information
type Base interface {
	// Namespace returns the current application namespace
	Namespace() string
}

// Limiter provides rate limiting functionality for HTTP and gRPC services
type Limiter interface {
	// HTTPRateLimit returns middleware for HTTP rate limiting
	HTTPRateLimit() middleware.Middleware
	// GRPCRateLimit returns middleware for gRPC rate limiting
	GRPCRateLimit() middleware.Middleware
}

// Registry provides service registration and discovery functionality
type Registry interface {
	// NewServiceRegistry creates a new service registrar
	NewServiceRegistry() registry.Registrar
	// NewServiceDiscovery creates a new service discovery client
	NewServiceDiscovery() registry.Discovery
}

// Router provides service routing functionality
type Router interface {
	// NewNodeRouter creates a new node filter for the specified service
	NewNodeRouter(serviceName string) selector.NodeFilter
}

// Config provides configuration management functionality
type Config interface {
	// Config retrieves configuration from a source using the specified file and group
	Config(fileName string, group string) (config.Source, error)
}

// LocalControlPlane provides a basic implementation of the ControlPlane interface
// for local development and testing purposes
type LocalControlPlane struct {
}

// HTTPRateLimit implements the Limiter interface for HTTP rate limiting
func (c *LocalControlPlane) HTTPRateLimit() middleware.Middleware {
	return nil
}

// GRPCRateLimit implements the Limiter interface for gRPC rate limiting
func (c *LocalControlPlane) GRPCRateLimit() middleware.Middleware {
	return nil
}

// NewServiceRegistry implements the Registry interface for service registration
func (c *LocalControlPlane) NewServiceRegistry() registry.Registrar {
	return nil
}

// NewServiceDiscovery implements the Registry interface for service discovery
func (c *LocalControlPlane) NewServiceDiscovery() registry.Discovery {
	return nil
}

// NewNodeRouter implements the Router interface for service routing
func (c *LocalControlPlane) NewNodeRouter(serviceName string) selector.NodeFilter {
	return nil
}

// Config implements the Config interface for configuration management
func (c *LocalControlPlane) Config(fileName string, group string) (config.Source, error) {
	return nil, nil
}

// Namespace implements the Base interface for namespace management
func (c *LocalControlPlane) Namespace() string {
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

	// Default configuration file name based on application name
	configFileName := fmt.Sprintf("%s.yaml", GetName())
	namespace := a.GetControlPlane().Namespace()

	// Log configuration loading attempt
	a.logHelper.Infof("Loading configuration - File: [%s] Group: [%s] Namespace: [%s]",
		configFileName, GetName(), namespace)

	// Get configuration source from control plane
	configSource, err := a.GetControlPlane().Config(configFileName, GetName())
	if err != nil {
		a.logHelper.Errorf("Failed to load configuration - File: [%s] Group: [%s] Namespace: [%s] Error: %v",
			configFileName, GetName(), namespace, err)
		return nil, fmt.Errorf("failed to load configuration source: %w", err)
	}

	// Create and load configuration
	cfg := config.New(config.WithSource(configSource))
	if cfg == nil {
		return nil, fmt.Errorf("failed to create configuration with source")
	}

	if err := cfg.Load(); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Set global configuration
	if err := a.SetGlobalConfig(cfg); err != nil {
		return nil, fmt.Errorf("failed to set global configuration: %w", err)
	}

	return cfg, nil
}

// GetServiceRegistry returns a new service registry instance
func GetServiceRegistry() (registry.Registrar, error) {
	if Lynx() == nil || Lynx().GetControlPlane() == nil {
		return nil, fmt.Errorf("control plane not initialized")
	}
	reg := Lynx().GetControlPlane().NewServiceRegistry()
	if reg == nil {
		return nil, fmt.Errorf("failed to create service registry")
	}
	return reg, nil
}

// GetServiceDiscovery returns a new service discovery instance
func GetServiceDiscovery() (registry.Discovery, error) {
	if Lynx() == nil || Lynx().GetControlPlane() == nil {
		return nil, fmt.Errorf("control plane not initialized")
	}
	disc := Lynx().GetControlPlane().NewServiceDiscovery()
	if disc == nil {
		return nil, fmt.Errorf("failed to create service discovery")
	}
	return disc, nil
}
