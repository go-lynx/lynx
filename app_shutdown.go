package lynx

import (
	"github.com/go-lynx/lynx/events"
	"github.com/go-lynx/lynx/log"
	"google.golang.org/grpc"
)

// Close gracefully shuts down the Lynx application.
func (a *LynxApp) Close() error {
	if a == nil {
		return nil
	}
	a.closeOnce.Do(func() {
		a.closeErr = a.close()
	})
	return a.closeErr
}

func (a *LynxApp) close() error {
	a.emitSystemEvent(events.EventSystemShutdown, map[string]any{
		"app_name":    a.name,
		"app_version": a.version,
		"host":        a.host,
		"reason":      "application_close",
	})

	if a.pluginManager != nil {
		a.pluginManager.UnloadPlugins()
		if rt := a.pluginManager.GetRuntime(); rt != nil {
			rt.Shutdown()
		}
	}

	a.grpcSubsMu.Lock()
	grpcSubsCopy := make(map[string]*grpc.ClientConn)
	for k, v := range a.grpcSubs {
		grpcSubsCopy[k] = v
	}
	a.grpcSubs = nil
	a.grpcSubsMu.Unlock()

	closeGrpcConnections(grpcSubsCopy)

	if a.eventListenerManager != nil {
		a.eventListenerManager.Clear()
	}
	if a.eventManager != nil {
		a.eventManager.StopHealthCheck()
		if err := a.eventManager.Close(); err != nil {
			log.Errorf("Failed to close app event bus: %v", err)
		}
	}

	if a.globalConf != nil {
		if err := a.globalConf.Close(); err != nil {
			log.Errorf("Failed to close global configuration: %v", err)
		}
		a.globalConf = nil
	}

	clearDefaultAppIf(a)
	cleanupMemoryStatsCache()
	resetInitState()

	return nil
}
