package app

import (
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/selector"
)

type ControlPlane interface {
	Base
	Limiter
	Registry
	Router
	Config
}

type Base interface {
	Namespace() string
}

type Limiter interface {
	HttpRateLimit() middleware.Middleware
	GrpcRateLimit() middleware.Middleware
}

type Registry interface {
	NewServiceRegistry() registry.Registrar
	NewServiceDiscovery() registry.Discovery
}

type Router interface {
	NewNodeRouter(name string) selector.NodeFilter
}

type Config interface {
	Config(fileName string, group string) (config.Source, error)
}

func ServiceRegistry() registry.Registrar {
	return Lynx().ControlPlane().NewServiceRegistry()
}

func ServiceDiscovery() registry.Discovery {
	return Lynx().ControlPlane().NewServiceDiscovery()
}

func (a *LynxApp) ControlPlane() ControlPlane {
	return Lynx().plane
}

func (a *LynxApp) SetControlPlane(plane ControlPlane) {
	Lynx().plane = plane
}

func (a *LynxApp) GetBootConfiguration() map[string]config.Value {
	if Lynx().ControlPlane() == nil {
		return make(map[string]config.Value)
	}
	yaml := Name() + ".yaml"
	Lynx().GetHelper().Infof("Reading from the configuration center,file:[%v] group:[%v] namespace:[%v]", yaml, Name(), Lynx().ControlPlane().Namespace())

	s, err := Lynx().ControlPlane().Config(yaml, Lynx().ControlPlane().Namespace())
	if err != nil {
		Lynx().GetHelper().Errorf("Failed to read the configuration file:[%v] group:[%v] namespace:[%v]", yaml, Name(), Lynx().ControlPlane().Namespace())
		panic(err)
	}
	c := config.New(config.WithSource(s))
	if err := c.Load(); err != nil {
		panic(err)
	}
	val, _ := c.Value("lynx").Map()
	return val
}
