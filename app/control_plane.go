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
// ControlPlane 定义了管理核心应用服务的接口，
// 这些服务包括限流、服务发现、路由和配置管理。
type ControlPlane interface {
	// SystemCore provides basic application information.
	// SystemCore 提供应用的基本信息。
	SystemCore
	// RateLimiter provides rate limiting functionality for HTTP and gRPC services.
	// RateLimiter 为 HTTP 和 gRPC 服务提供限流功能。
	RateLimiter
	// ServiceRegistry provides service registration and discovery functionality.
	// ServiceRegistry 提供服务注册和发现功能。
	ServiceRegistry
	// RouteManager provides service routing functionality.
	// RouteManager 提供服务路由功能。
	RouteManager
	// ConfigManager provides configuration management functionality.
	// ConfigManager 提供配置管理功能。
	ConfigManager
}

// SystemCore provides basic application information
// SystemCore 提供应用的基本信息。
type SystemCore interface {
	// GetNamespace returns the current application control plane namespace
	// GetNamespace 返回当前应用控制平面的命名空间。
	GetNamespace() string
}

// RateLimiter provides rate limiting functionality for HTTP and gRPC services
// RateLimiter 为 HTTP 和 gRPC 服务提供限流功能。
type RateLimiter interface {
	// HTTPRateLimit returns middleware for HTTP rate limiting
	// HTTPRateLimit 返回用于 HTTP 限流的中间件。
	HTTPRateLimit() middleware.Middleware
	// GRPCRateLimit returns middleware for gRPC rate limiting
	// GRPCRateLimit 返回用于 gRPC 限流的中间件。
	GRPCRateLimit() middleware.Middleware
}

// ServiceRegistry provides service registration and discovery functionality
// ServiceRegistry 提供服务注册和发现功能。
type ServiceRegistry interface {
	// NewServiceRegistry creates a new service registrar
	// NewServiceRegistry 创建一个新的服务注册器。
	NewServiceRegistry() registry.Registrar
	// NewServiceDiscovery creates a new service discovery client
	// NewServiceDiscovery 创建一个新的服务发现客户端。
	NewServiceDiscovery() registry.Discovery
}

// RouteManager provides service routing functionality
// RouteManager 提供服务路由功能。
type RouteManager interface {
	// NewNodeRouter creates a new node filter for the specified service
	// NewNodeRouter 为指定的服务创建一个新的节点过滤器。
	NewNodeRouter(serviceName string) selector.NodeFilter
}

// ConfigManager provides configuration management functionality
// ConfigManager 提供配置管理功能。
type ConfigManager interface {
	// GetConfig retrieves configuration from a source using the specified file and group
	// GetConfig 使用指定的文件和组从源中获取配置。
	GetConfig(fileName string, group string) (config.Source, error)
}

// DefaultControlPlane provides a basic implementation of the ControlPlane interface
// for local development and testing purposes
// DefaultControlPlane 为本地开发和测试提供 ControlPlane 接口的基础实现。
type DefaultControlPlane struct {
}

// HTTPRateLimit implements the RateLimiter interface for HTTP rate limiting
// HTTPRateLimit 实现 RateLimiter 接口，用于 HTTP 限流。
func (c *DefaultControlPlane) HTTPRateLimit() middleware.Middleware {
	return nil
}

// GRPCRateLimit implements the RateLimiter interface for gRPC rate limiting
// GRPCRateLimit 实现 RateLimiter 接口，用于 gRPC 限流。
func (c *DefaultControlPlane) GRPCRateLimit() middleware.Middleware {
	return nil
}

// NewServiceRegistry implements the ServiceRegistry interface for service registration
// NewServiceRegistry 实现 ServiceRegistry 接口，用于服务注册。
func (c *DefaultControlPlane) NewServiceRegistry() registry.Registrar {
	return nil
}

// NewServiceDiscovery implements the ServiceRegistry interface for service discovery
// NewServiceDiscovery 实现 ServiceRegistry 接口，用于服务发现。
func (c *DefaultControlPlane) NewServiceDiscovery() registry.Discovery {
	return nil
}

// NewNodeRouter implements the RouteManager interface for service routing
// NewNodeRouter 实现 RouteManager 接口，用于服务路由。
func (c *DefaultControlPlane) NewNodeRouter(serviceName string) selector.NodeFilter {
	return nil
}

// GetConfig implements the ConfigManager interface for configuration management
// GetConfig 实现 ConfigManager 接口，用于配置管理。
func (c *DefaultControlPlane) GetConfig(fileName string, group string) (config.Source, error) {
	return nil, nil
}

// GetNamespace implements the SystemCore interface for namespace management
// GetNamespace 实现 SystemCore 接口，用于命名空间管理。
func (c *DefaultControlPlane) GetNamespace() string {
	return ""
}

// GetControlPlane returns the current control plane instance
// GetControlPlane 返回当前的控制平面实例。
func (a *LynxApp) GetControlPlane() ControlPlane {
	if a == nil {
		return nil
	}
	return Lynx().controlPlane
}

// SetControlPlane sets the control plane instance for the application
// SetControlPlane 为应用设置控制平面实例。
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
// InitControlPlaneConfig 初始化控制平面配置。
// 它从指定源加载配置并设置全局配置。
func (a *LynxApp) InitControlPlaneConfig() (config.Config, error) {
	if a == nil {
		return nil, fmt.Errorf("lynx app instance is nil")
	}

	// Create new configuration if control plane is not initialized
	// 如果控制平面未初始化，则创建新配置。
	if a.GetControlPlane() == nil {
		cfg := config.New()
		if cfg == nil {
			return nil, fmt.Errorf("failed to create new configuration")
		}
		return cfg, nil
	}

	// Default configuration file name based on application name
	// 基于应用名称生成默认配置文件名。
	configFileName := fmt.Sprintf("%s.yaml", GetName())
	namespace := a.GetControlPlane().GetNamespace()

	// Log configuration loading attempt
	// 记录配置加载尝试信息。
	log.Infof("Loading configuration - File: [%s] Group: [%s] GetNamespace: [%s]",
		configFileName, GetName(), namespace)

	// Get configuration source from control plane
	// 从控制平面获取配置源。
	configSource, err := a.GetControlPlane().GetConfig(configFileName, GetName())
	if err != nil {
		log.Errorf("Failed to load configuration - File: [%s] Group: [%s] GetNamespace: [%s] Error: %v",
			configFileName, GetName(), namespace, err)
		return nil, fmt.Errorf("failed to load configuration source: %w", err)
	}

	// Create and load configuration
	// 创建并加载配置。
	cfg := config.New(config.WithSource(configSource))
	if cfg == nil {
		return nil, fmt.Errorf("failed to create configuration with source")
	}

	// Load configuration
	// 加载配置。
	if err := cfg.Load(); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Set global configuration
	// 设置全局配置。
	if err := a.SetGlobalConfig(cfg); err != nil {
		return nil, fmt.Errorf("failed to set global configuration: %w", err)
	}

	return cfg, nil
}

// GetServiceRegistry returns a new service registry instance
// GetServiceRegistry 返回一个新的服务注册实例。
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
// GetServiceDiscovery 返回一个新的服务发现实例。
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
