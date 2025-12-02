// Package lynx provides the core application framework for building microservices.
//
// Lynx is a plug-and-play Go microservices framework built on top of Kratos,
// providing a unified runtime environment for plugin-based application development.
//
// # Architecture
//
// The framework is organized around the following core concepts:
//
//   - LynxApp: The main application instance managing lifecycle and configuration
//   - Plugin Manager: Handles plugin registration, dependency resolution, and lifecycle
//   - Runtime: Provides resource sharing, event handling, and configuration access
//   - Control Plane: Manages service discovery, rate limiting, and routing
//
// # File Organization
//
// The root package contains the following files:
//
//   - app.go: LynxApp core structure, initialization, and main API
//   - manager.go: Plugin manager interfaces and implementation
//   - lifecycle.go: Plugin lifecycle operations (init/start/stop)
//   - ops.go: Plugin loading and unloading operations
//   - topology.go: Plugin dependency resolution and ordering
//   - runtime.go: Runtime plugin providing resource and event management
//   - controlplane.go: Control plane interfaces for service management
//   - certificate.go: TLS certificate provider interface
//   - prepare.go: Plugin preparation and bootstrapping from configuration
//   - recovery.go: Error recovery and resilience mechanisms
//
// # Quick Start
//
// Basic usage of the Lynx framework:
//
//	package main
//
//	import (
//	    "github.com/go-kratos/kratos/v2/config"
//	    "github.com/go-kratos/kratos/v2/config/file"
//	    "github.com/go-lynx/lynx"
//	    "github.com/go-lynx/lynx/boot"
//	)
//
//	func main() {
//	    // Load configuration
//	    cfg := config.New(config.WithSource(file.NewSource("config.yaml")))
//	    cfg.Load()
//
//	    // Create application with plugins
//	    app, err := lynx.NewApp(cfg)
//	    if err != nil {
//	        panic(err)
//	    }
//	    defer app.Close()
//
//	    // Load and start plugins
//	    if err := app.GetPluginManager().LoadPlugins(cfg); err != nil {
//	        panic(err)
//	    }
//
//	    // Run application using boot package
//	    boot.Run(app)
//	}
//
// # Plugin Development
//
// To create a custom plugin, implement the plugins.Plugin interface:
//
//	type MyPlugin struct {
//	    *plugins.BasePlugin
//	}
//
//	func (p *MyPlugin) Name() string { return "my-plugin" }
//	func (p *MyPlugin) ID() string   { return "my-plugin-v1" }
//
//	func (p *MyPlugin) Initialize(plugin plugins.Plugin, rt plugins.Runtime) error {
//	    // Initialization logic
//	    return nil
//	}
//
//	func (p *MyPlugin) Start(plugin plugins.Plugin) error {
//	    // Startup logic
//	    return nil
//	}
//
//	func (p *MyPlugin) Stop(plugin plugins.Plugin) error {
//	    // Shutdown logic
//	    return nil
//	}
//
// # Configuration
//
// The framework uses YAML configuration files. See conf/boot-example.yml for
// a complete configuration reference.
//
// # For More Information
//
// Visit the official documentation at https://go-lynx.cn/docs
package lynx
