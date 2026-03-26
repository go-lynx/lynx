package lynx

import (
	"fmt"
	"sync"

	"github.com/go-lynx/lynx/events"
	"github.com/go-lynx/lynx/plugins"
)

var (
	// lynxApp is the singleton instance of the Lynx application.
	lynxApp *LynxApp
	// lynxMu protects process-wide singleton compatibility access.
	lynxMu sync.RWMutex
)

// SetDefaultApp publishes app as the process-wide default Lynx application instance.
func SetDefaultApp(app *LynxApp) {
	lynxMu.Lock()
	defer lynxMu.Unlock()
	lynxApp = app
	if app == nil {
		events.ClearDefaultEventBusProvider()
		events.ClearDefaultListenerManagerProvider()
		return
	}
	events.SetDefaultEventBusProvider(func() *events.EventBusManager {
		if lynxApp == nil {
			return nil
		}
		return lynxApp.eventManager
	})
	events.SetDefaultListenerManagerProvider(func() *events.EventListenerManager {
		if lynxApp == nil {
			return nil
		}
		return lynxApp.eventListenerManager
	})
}

// ClearDefaultApp clears the process-wide default Lynx application instance.
func ClearDefaultApp() {
	SetDefaultApp(nil)
}

// clearDefaultAppIf clears the global default only when it still points to app.
func clearDefaultAppIf(app *LynxApp) bool {
	lynxMu.Lock()
	defer lynxMu.Unlock()
	if lynxApp != app {
		return false
	}
	lynxApp = nil
	events.ClearDefaultEventBusProvider()
	events.ClearDefaultListenerManagerProvider()
	return true
}

// Lynx returns the process-wide default LynxApp instance.
//
// Deprecated: prefer passing an explicit *LynxApp and using instance helpers.
func Lynx() *LynxApp {
	lynxMu.RLock()
	defer lynxMu.RUnlock()
	return lynxApp
}

// GetTypedPluginFromApp retrieves a typed plugin from an explicit app instance.
func GetTypedPluginFromApp[T plugins.Plugin](app *LynxApp, name string) (T, error) {
	var zero T
	if app == nil {
		return zero, fmt.Errorf("lynx application not initialized")
	}

	manager := app.GetTypedPluginManager()
	if manager == nil {
		return zero, fmt.Errorf("typed plugin manager not initialized")
	}

	return GetTypedPluginFromManager[T](manager, name)
}

// MustGetTypedPluginFromApp retrieves a typed plugin from an explicit app instance or panics.
func MustGetTypedPluginFromApp[T plugins.Plugin](app *LynxApp, name string) T {
	p, err := GetTypedPluginFromApp[T](app, name)
	if err != nil {
		panic(err)
	}
	return p
}

// GetTypedPlugin globally retrieves a type-safe plugin instance.
// Deprecated: prefer GetTypedPluginFromApp or GetTypedPluginFromManager.
func GetTypedPlugin[T plugins.Plugin](name string) (T, error) {
	return GetTypedPluginFromApp[T](Lynx(), name)
}

// Shutdown is an alias for Close for backward compatibility.
func (a *LynxApp) Shutdown() error {
	return a.Close()
}
