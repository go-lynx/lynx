package lynx

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-lynx/lynx/conf"
	"github.com/go-lynx/lynx/log"
	"github.com/go-lynx/lynx/subscribe"
	"google.golang.org/grpc"
)

// LoadPlugins loads plugins through the app-owned plugin manager and then wires
// application-level subscriptions that depend on started plugins/control plane.
func (a *LynxApp) LoadPlugins() error {
	if a == nil {
		return fmt.Errorf("lynx app instance is nil")
	}
	pm := a.GetPluginManager()
	if pm == nil {
		return fmt.Errorf("plugin manager is nil")
	}
	if a.globalConf == nil {
		return fmt.Errorf("global configuration is nil")
	}

	if err := pm.LoadPlugins(a.globalConf); err != nil {
		return err
	}

	if err := a.configureGrpcSubscriptions(); err != nil {
		pm.UnloadPlugins()
		return err
	}

	return nil
}

// LoadPluginsByName loads a subset of plugins through the app-owned plugin manager.
func (a *LynxApp) LoadPluginsByName(names []string) error {
	if a == nil {
		return fmt.Errorf("lynx app instance is nil")
	}
	pm := a.GetPluginManager()
	if pm == nil {
		return fmt.Errorf("plugin manager is nil")
	}
	if a.globalConf == nil {
		return fmt.Errorf("global configuration is nil")
	}

	if err := pm.LoadPluginsByName(a.globalConf, names); err != nil {
		return err
	}

	if err := a.configureGrpcSubscriptions(); err != nil {
		pm.UnloadPluginsByName(names)
		return err
	}

	return nil
}

func (a *LynxApp) configureGrpcSubscriptions() error {
	if a == nil || a.bootConfig == nil || a.bootConfig.Lynx == nil || a.bootConfig.Lynx.Subscriptions == nil {
		a.replaceGrpcSubscriptions(nil)
		return nil
	}

	subs := a.bootConfig.Lynx.Subscriptions
	if len(subs.GetGrpc()) == 0 {
		a.replaceGrpcSubscriptions(nil)
		return nil
	}

	controlPlane := a.GetControlPlane()
	if controlPlane == nil {
		return fmt.Errorf("grpc subscriptions configured but control plane is not available (install a control plane plugin)")
	}

	disc := controlPlane.NewServiceDiscovery()
	if disc == nil {
		return fmt.Errorf("grpc subscriptions configured but service discovery is not available")
	}

	routerFactory := func(service string) selector.NodeFilter {
		return controlPlane.NewNodeRouter(service)
	}

	var tlsProviders *subscribe.ClientTLSProviders
	if hasTLSSubscription(subs) {
		tlsProviders = &subscribe.ClientTLSProviders{
			ConfigProvider: controlPlane.GetConfig,
			DefaultRootCA:  a.defaultRootCAProvider(),
		}
	}

	conns, err := subscribe.BuildGrpcSubscriptions(subs, disc, routerFactory, tlsProviders)
	if err != nil {
		closeGrpcConnections(conns)
		return fmt.Errorf("build grpc subscriptions failed: %w", err)
	}

	a.replaceGrpcSubscriptions(conns)
	return nil
}

func hasTLSSubscription(subs *conf.Subscriptions) bool {
	if subs == nil || len(subs.GetGrpc()) == 0 {
		return false
	}
	for _, g := range subs.GetGrpc() {
		if g.GetTls() {
			return true
		}
	}
	return false
}

func (a *LynxApp) defaultRootCAProvider() func() []byte {
	return func() []byte {
		if a == nil || a.Certificate() == nil {
			return nil
		}
		return a.Certificate().GetRootCACertificate()
	}
}

func (a *LynxApp) replaceGrpcSubscriptions(conns map[string]*grpc.ClientConn) {
	if a == nil {
		return
	}

	next := make(map[string]*grpc.ClientConn, len(conns))
	for name, conn := range conns {
		next[name] = conn
	}

	a.grpcSubsMu.Lock()
	prev := a.grpcSubs
	a.grpcSubs = next
	a.grpcSubsMu.Unlock()

	for name, oldConn := range prev {
		newConn, stillPresent := next[name]
		if stillPresent && newConn == oldConn {
			continue
		}
		if oldConn != nil {
			if err := oldConn.Close(); err != nil {
				log.Errorf("Failed to close previous gRPC connection for service %s: %v", name, err)
			}
		}
	}
}

func closeGrpcConnections(conns map[string]*grpc.ClientConn) {
	for name, conn := range conns {
		if conn == nil {
			continue
		}
		if err := conn.Close(); err != nil {
			log.Errorf("Failed to close gRPC connection for service %s: %v", name, err)
		}
	}
}
