package lynx

import (
	"fmt"
	"os"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/conf"
	"github.com/go-lynx/lynx/events"
	"github.com/go-lynx/lynx/plugins"
	"google.golang.org/grpc"
)

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
