// Package lynx provides the core application framework for building Go microservices.
//
// The implementation lives in internal/app; this file and facade.go re-export the
// public API so that callers continue to use import paths of the form
// github.com/go-lynx/lynx.
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
// # Package Organization
//
// The root package is a thin facade over internal/app. All implementation lives in:
//
//   - internal/app/: Core implementation (LynxApp, managers, lifecycle, etc.)
//   - internal/app/compat/: Deprecated TypedRuntimePlugin wrapper (removed in v2.0)
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
//	    app, err := lynx.NewStandaloneApp(cfg)
//	    if err != nil {
//	        panic(err)
//	    }
//	    defer app.Close()
//
//	    // Load and start plugins
//	    if err := app.LoadPlugins(); err != nil {
//	        panic(err)
//	    }
//
//	    // Or hand the app to the optional boot package
//	    // boot.NewApplication(wire).Run()
//	}
//
// # For More Information
//
// Visit the official documentation at https://go-lynx.cn/docs
package lynx
