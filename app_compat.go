package lynx

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-lynx/lynx/events"
	"github.com/go-lynx/lynx/log"
	"github.com/go-lynx/lynx/plugins"
)

var (
	// lynxApp is the singleton instance of the Lynx application.
	lynxApp *LynxApp
	// lynxMu protects process-wide singleton compatibility access.
	lynxMu sync.RWMutex

	// initMu protects default-app initialization state.
	initMu sync.Mutex
	// initErr stores the last initialization error.
	initErr error
	// initCompleted indicates whether at least one initialization attempt finished.
	initCompleted bool
	// initInProgress indicates whether an initialization attempt is currently running.
	initInProgress bool
	// initDone channel signals current initialization attempt completion.
	initDone chan struct{}
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

// resetInitState resets compatibility initialization state (for testing/restart scenarios).
// Should only be called during application shutdown.
func resetInitState() {
	initMu.Lock()
	defer initMu.Unlock()
	initErr = nil
	initCompleted = false
	initInProgress = false
	initDone = nil
}

// getInitTimeout returns the compatibility singleton initialization timeout, default 30s.
// Can be configured via "lynx.app.init_timeout" config key.
func getInitTimeout(cfg config.Config) time.Duration {
	defaultTimeout := 30 * time.Second
	if cfg == nil {
		return defaultTimeout
	}

	var confStr string
	if err := cfg.Value("lynx.app.init_timeout").Scan(&confStr); err == nil {
		if parsed, err2 := time.ParseDuration(confStr); err2 == nil {
			if parsed < 10*time.Second {
				log.Warnf("init_timeout too short (%v), using minimum 10s", parsed)
				return 10 * time.Second
			}
			if parsed > 300*time.Second {
				log.Warnf("init_timeout too long (%v), using maximum 300s", parsed)
				return 300 * time.Second
			}
			return parsed
		}
	}
	return defaultTimeout
}

// NewApp creates or returns the process-wide default Lynx application instance.
// Deprecated: prefer NewStandaloneApp plus explicit SetDefaultApp only at the shell boundary.
//
// This constructor is retained as a compatibility entrypoint. The actual core
// instance construction stays in NewStandaloneApp/initializeApp so the happy
// path can remain instance-oriented.
func NewApp(cfg config.Config, plugins ...plugins.Plugin) (*LynxApp, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}

	initTimeout := getInitTimeout(cfg)

	for {
		if existing := Lynx(); existing != nil {
			log.Warnf("Lynx application already initialized, returning existing instance. New configuration and plugins are ignored.")
			return existing, nil
		}

		initMu.Lock()
		if initInProgress {
			doneChan := initDone
			initMu.Unlock()
			if doneChan == nil {
				return nil, fmt.Errorf("initialization in progress but completion channel is nil")
			}
			select {
			case <-doneChan:
				continue
			case <-time.After(initTimeout):
				return nil, fmt.Errorf("initialization timeout: initialization did not complete within %v", initTimeout)
			}
		}

		initInProgress = true
		initCompleted = false
		initErr = nil
		doneChan := make(chan struct{})
		initDone = doneChan
		initMu.Unlock()

		app, err := NewStandaloneApp(cfg, plugins...)

		initMu.Lock()
		if err != nil {
			initErr = err
		} else {
			SetDefaultApp(app)
		}
		initCompleted = true
		initInProgress = false
		close(doneChan)
		initMu.Unlock()

		if err != nil {
			return nil, fmt.Errorf("failed to initialize application: %w", err)
		}
		if app == nil {
			return nil, fmt.Errorf("application initialization resulted in nil instance")
		}
		return app, nil
	}
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

// MustGetTypedPluginFromApp retrieves a typed plugin from an explicit app instance.
// It panics if the plugin cannot be found or is not of the expected type.
// This function is intended for use in application startup code where a missing
// plugin is a fatal misconfiguration. For runtime lookups where the plugin may
// legitimately be absent, prefer GetTypedPluginFromApp instead.
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

// GetName returns the process-wide default app name for backward compatibility.
func GetName() string {
	if app := Lynx(); app != nil {
		return app.Name()
	}
	return ""
}

// GetHost returns the process-wide default app host for backward compatibility.
func GetHost() string {
	if app := Lynx(); app != nil {
		return app.Host()
	}
	return ""
}

// GetVersion returns the process-wide default app version for backward compatibility.
func GetVersion() string {
	if app := Lynx(); app != nil {
		return app.Version()
	}
	return ""
}

// GetServiceRegistry returns a service registrar from the default app for backward compatibility.
func GetServiceRegistry() (registry.Registrar, error) {
	if app := Lynx(); app != nil {
		return app.GetServiceRegistry()
	}
	return nil, nil
}

// GetServiceDiscovery returns a service discovery client from the default app for backward compatibility.
func GetServiceDiscovery() (registry.Discovery, error) {
	if app := Lynx(); app != nil {
		return app.GetServiceDiscovery()
	}
	return nil, nil
}

// Shutdown is an alias for Close for backward compatibility.
func (a *LynxApp) Shutdown() error {
	return a.Close()
}
