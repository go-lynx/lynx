//go:build !v2

// Compatibility layer — deprecated symbols retained for migration.
// Will be removed in v2.0.
package lynx

import (
	iapp "github.com/go-lynx/lynx/internal/app"
	icompat "github.com/go-lynx/lynx/internal/app/compat"
	"github.com/go-lynx/lynx/plugins"
)

// TypedRuntimePlugin and RuntimePlugin are in the compat package.
type TypedRuntimePlugin = icompat.TypedRuntimePlugin
type RuntimePlugin = icompat.RuntimePlugin

var NewTypedRuntimePlugin = icompat.NewTypedRuntimePlugin
var NewRuntimePlugin = icompat.NewRuntimePlugin

func GetTypedResource[T any](r *TypedRuntimePlugin, name string) (T, error) {
	return icompat.GetTypedResource[T](r, name)
}

func RegisterTypedResource[T any](r *TypedRuntimePlugin, name string, resource T) error {
	return icompat.RegisterTypedResource[T](r, name, resource)
}

// Re-export compat app functions (defined in internal/app/app_compat.go)
var SetDefaultApp = iapp.SetDefaultApp
var ClearDefaultApp = iapp.ClearDefaultApp
var Lynx = iapp.Lynx
var NewApp = iapp.NewApp
var GetName = iapp.GetName
var GetHost = iapp.GetHost
var GetVersion = iapp.GetVersion
var GetServiceRegistry = iapp.GetServiceRegistry
var GetServiceDiscovery = iapp.GetServiceDiscovery

func GetTypedPlugin[T plugins.Plugin](name string) (T, error) {
	return iapp.GetTypedPlugin[T](name)
}

func GetTypedPluginFromApp[T plugins.Plugin](app *LynxApp, name string) (T, error) {
	return iapp.GetTypedPluginFromApp[T](app, name)
}

func MustGetTypedPluginFromApp[T plugins.Plugin](app *LynxApp, name string) T {
	return iapp.MustGetTypedPluginFromApp[T](app, name)
}

// TypedPluginManager is a deprecated alias for PluginManager.
type TypedPluginManager = iapp.TypedPluginManager

// ConfigReloadPlan is retained only as a compatibility report for older callers.
type ConfigReloadPlan = iapp.ConfigReloadPlan
