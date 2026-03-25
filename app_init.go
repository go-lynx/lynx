package lynx

import (
	"fmt"
	"os"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/conf"
	"github.com/go-lynx/lynx/events"
	"github.com/go-lynx/lynx/log"
	"github.com/go-lynx/lynx/plugins"
	"google.golang.org/grpc"
)

// resetInitState resets initialization state (for testing/restart scenarios)
// Should only be called during application shutdown.
func resetInitState() {
	initMu.Lock()
	defer initMu.Unlock()
	initErr = nil
	initCompleted = false
	initInProgress = false
	initDone = nil
}

// getInitTimeout returns the initialization timeout from config, default 30s.
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

// NewStandaloneApp creates a fully initialized Lynx application instance without
// publishing it as the process-wide default singleton.
//
// This is the preferred constructor for tests, isolated runtimes, and future
// multi-instance scenarios. Call SetDefaultApp explicitly only when a global
// default instance is truly needed.
func NewStandaloneApp(cfg config.Config, plugins ...plugins.Plugin) (*LynxApp, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}
	return initializeApp(cfg, plugins...)
}

// NewApp creates or returns the process-wide default Lynx application instance.
// It preserves the historical singleton behavior for compatibility, while the
// actual instance construction is delegated to NewStandaloneApp/initializeApp.
//
// NewApp uses an explicit initialization state machine instead of sync.Once so
// failed initialization can be retried and concurrent callers can wait on the
// in-flight attempt with a bounded timeout.
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

// initializeApp handles the actual initialization of the LynxApp instance.
func initializeApp(cfg config.Config, plugins ...plugins.Plugin) (*LynxApp, error) {
	var bConf conf.Bootstrap
	if err := cfg.Scan(&bConf); err != nil {
		return nil, fmt.Errorf("failed to parse bootstrap configuration: %w", err)
	}

	if bConf.Lynx == nil || bConf.Lynx.Application == nil {
		return nil, fmt.Errorf("invalid bootstrap configuration: missing required fields")
	}

	host := bConf.Lynx.Application.Host
	if host == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("failed to get hostname: %w", err)
		}
		host = hostname
	}

	typedMgr := NewTypedPluginManager(plugins...)
	typedMgr.SetConfig(cfg)
	eventManager, err := events.NewEventBusManager(events.DefaultBusConfigs())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize app event system: %w", err)
	}
	eventListenerManager := events.NewEventListenerManagerWithEventBus(eventManager)
	eventAdapter := events.NewPluginEventBusAdapterWithListenerManager(eventManager, eventListenerManager)
	app := &LynxApp{
		host:                 host,
		name:                 bConf.Lynx.Application.Name,
		version:              bConf.Lynx.Application.Version,
		bootConfig:           &bConf,
		globalConf:           cfg,
		pluginManager:        typedMgr,
		controlPlane:         &DefaultControlPlane{},
		grpcSubs:             make(map[string]*grpc.ClientConn),
		eventManager:         eventManager,
		eventListenerManager: eventListenerManager,
		eventAdapter:         eventAdapter,
	}
	app.eventManager.StartHealthCheck(30 * time.Second)
	app.injectRuntimeEventAdapter()

	if app.name == "" {
		return nil, fmt.Errorf("application name cannot be empty")
	}

	app.emitSystemEvent(events.EventSystemStart, map[string]any{
		"app_name":    app.name,
		"app_version": app.version,
		"host":        app.host,
	})

	return app, nil
}
