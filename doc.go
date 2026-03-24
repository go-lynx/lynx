// Package lynx provides the core plugin orchestration framework used by Lynx applications.
//
// Lynx is built around plugin orchestration, dependency-aware lifecycle
// management, and a unified runtime for plugin-based applications built on top
// of Kratos.
//
// The root package should be understood primarily as framework core. Optional
// bootstrap/process helpers live in the boot package, while some singleton and
// legacy integration surfaces remain for compatibility.
//
// # Architecture
//
// The framework is organized around the following core concepts:
//
//   - LynxApp: Application-facing assembly for the core runtime and plugin manager
//   - Plugin Manager: Handles plugin registration, dependency resolution, and lifecycle
//   - UnifiedRuntime: Provides resource sharing, event handling, and plugin-scoped runtime views
//   - Control Plane: Optional shell-facing integration for discovery, routing, and config sources
//
// # File Organization
//
// The root package contains the following files:
//
//   - app.go: App instance assembly, singleton compatibility, runtime wiring, and shutdown
//   - manager.go: Plugin manager interfaces and implementation
//   - lifecycle.go: Plugin lifecycle operations (init/start/stop)
//   - ops.go: Plugin loading and unloading operations
//   - topology.go: Plugin dependency resolution and ordering
//   - runtime.go: Backward-compatible runtime wrapper around plugins.UnifiedRuntime
//   - controlplane.go: Optional shell-facing control plane interfaces
//   - certificate.go: TLS certificate provider interface
//   - prepare.go: Plugin preparation and bootstrapping from configuration
//   - recovery.go: Error recovery and resilience mechanisms
//
// # Quick Start
//
// Basic usage of the Lynx core:
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
//	    // Or hand the app to the optional boot package
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
// The framework uses YAML configuration files. Configuration changes that affect
// loaded plugins are expected to be applied by restart or external rollout
// tooling rather than in-process orchestration by the core.
//
// # For More Information
//
// Visit the official documentation at https://go-lynx.cn/docs
package lynx
