//go:build !v2

// Compatibility layer — process-wide singleton helpers.
//
// This package re-exports the singleton helpers from internal/app for use
// by the root lynx package compat.go. All symbols are deprecated and will
// be removed in v2.0.
//
// The implementation lives in internal/app/app_compat.go; this file is a
// thin forwarding layer so the root compat.go can import from a stable path.
package compat

import (
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/registry"
	iapp "github.com/go-lynx/lynx/internal/app"
	"github.com/go-lynx/lynx/plugins"
)

// Lynx returns the process-wide default LynxApp instance.
//
// Deprecated: prefer passing an explicit *iapp.LynxApp and using instance helpers.
func Lynx() *iapp.LynxApp {
	return iapp.Lynx()
}

// NewApp creates or returns the process-wide default Lynx application instance.
// Deprecated: prefer NewStandaloneApp plus explicit SetDefaultApp only at the shell boundary.
func NewApp(cfg config.Config, plugins ...plugins.Plugin) (*iapp.LynxApp, error) {
	return iapp.NewApp(cfg, plugins...)
}

// GetTypedPluginFromApp retrieves a typed plugin from an explicit app instance.
func GetTypedPluginFromApp[T plugins.Plugin](app *iapp.LynxApp, name string) (T, error) {
	return iapp.GetTypedPluginFromApp[T](app, name)
}

// MustGetTypedPluginFromApp retrieves a typed plugin from an explicit app instance.
// Panics if the plugin cannot be found or is not of the expected type.
func MustGetTypedPluginFromApp[T plugins.Plugin](app *iapp.LynxApp, name string) T {
	return iapp.MustGetTypedPluginFromApp[T](app, name)
}

// GetTypedPlugin globally retrieves a type-safe plugin instance.
// Deprecated: prefer GetTypedPluginFromApp or GetTypedPluginFromManager.
func GetTypedPlugin[T plugins.Plugin](name string) (T, error) {
	return iapp.GetTypedPlugin[T](name)
}

// GetName returns the process-wide default app name for backward compatibility.
func GetName() string {
	return iapp.GetName()
}

// GetHost returns the process-wide default app host for backward compatibility.
func GetHost() string {
	return iapp.GetHost()
}

// GetVersion returns the process-wide default app version for backward compatibility.
func GetVersion() string {
	return iapp.GetVersion()
}

// GetServiceRegistry returns a service registrar from the default app for backward compatibility.
func GetServiceRegistry() (registry.Registrar, error) {
	return iapp.GetServiceRegistry()
}

// GetServiceDiscovery returns a service discovery client from the default app for backward compatibility.
func GetServiceDiscovery() (registry.Discovery, error) {
	return iapp.GetServiceDiscovery()
}
