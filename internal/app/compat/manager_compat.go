//go:build !v2

// Compatibility layer — plugin manager aliases.
package compat

import (
	iapp "github.com/go-lynx/lynx/internal/app"
)

// TypedPluginManager is a deprecated alias for PluginManager.
// Deprecated: use PluginManager directly. Will be removed in v2.0.
type TypedPluginManager = iapp.PluginManager
