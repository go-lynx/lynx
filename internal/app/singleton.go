package app

import (
	"sync"

	"github.com/go-lynx/lynx/events"
)

// defaultApp is the process-wide singleton LynxApp, used only by the compat layer.
var defaultApp *LynxApp
var defaultAppMu sync.RWMutex

// SetDefaultApp publishes app as the process-wide default Lynx application instance.
func SetDefaultApp(app *LynxApp) {
	defaultAppMu.Lock()
	defer defaultAppMu.Unlock()
	defaultApp = app
	if app == nil {
		events.ClearDefaultEventBusProvider()
		events.ClearDefaultListenerManagerProvider()
		return
	}
	published := app
	events.SetDefaultEventBusProvider(func() *events.EventBusManager {
		if published == nil {
			return nil
		}
		return published.eventManager
	})
	events.SetDefaultListenerManagerProvider(func() *events.EventListenerManager {
		if published == nil {
			return nil
		}
		return published.eventListenerManager
	})
}

// ClearDefaultApp clears the process-wide default Lynx application instance.
func ClearDefaultApp() {
	SetDefaultApp(nil)
}

// GetDefaultApp returns the process-wide default LynxApp, or nil.
func GetDefaultApp() *LynxApp {
	defaultAppMu.RLock()
	defer defaultAppMu.RUnlock()
	return defaultApp
}

// clearDefaultAppIf clears the global default only when it still points to app.
// Called by shutdown.go during Close().
func clearDefaultAppIf(app *LynxApp) bool {
	defaultAppMu.Lock()
	defer defaultAppMu.Unlock()
	if defaultApp != app {
		return false
	}
	defaultApp = nil
	events.ClearDefaultEventBusProvider()
	events.ClearDefaultListenerManagerProvider()
	return true
}

// appShutdownHook is called after every LynxApp.Close() completes.
// Default is a no-op; the compat layer registers its own cleanup via SetAppShutdownHook.
var appShutdownHook func() = func() {}

// SetAppShutdownHook replaces the post-close hook. Safe to call at init time.
func SetAppShutdownHook(fn func()) {
	if fn == nil {
		appShutdownHook = func() {}
		return
	}
	appShutdownHook = fn
}
