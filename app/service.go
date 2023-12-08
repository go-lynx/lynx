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

type DefaultControlPlane struct {
}

func (c *DefaultControlPlane) HttpRateLimit() middleware.Middleware {
	return nil
}

func (c *DefaultControlPlane) GrpcRateLimit() middleware.Middleware {
	return nil
}

func (c *DefaultControlPlane) NewServiceRegistry() registry.Registrar {
	return nil
}

func (c *DefaultControlPlane) NewServiceDiscovery() registry.Discovery {
	return nil
}

func (c *DefaultControlPlane) NewNodeRouter(name string) selector.NodeFilter {
	return nil
}

func (c *DefaultControlPlane) Config(fileName string, group string) (config.Source, error) {
	return nil, nil
}

func (c *DefaultControlPlane) Namespace() string {
	return ""
}

func (a *LynxApp) ControlPlane() ControlPlane {
	return Lynx().controlPlane
}

func (a *LynxApp) SetControlPlane(plane ControlPlane) {
	Lynx().controlPlane = plane
}

func (a *LynxApp) ControlPlaneBootConfiguration() config.Config {
	if Lynx().ControlPlane() == nil {
		return config.New()
	}

	yaml := Name() + ".yaml"
	Lynx().GetHelper().Infof("Reading from the configuration center,file:[%v] group:[%v] namespace:[%v]", yaml, Name(), Lynx().ControlPlane().Namespace())
	s, err := Lynx().ControlPlane().Config(yaml, Name())
	if err != nil {
		Lynx().GetHelper().Errorf("Failed to read the configuration file:[%v] group:[%v] namespace:[%v]", yaml, Name(), Lynx().ControlPlane().Namespace())
		panic(err)
	}

	c := config.New(config.WithSource(s))
	if err := c.Load(); err != nil {
		panic(err)
	}
	a.setGlobalConfig(c)
	return c
}

func ServiceRegistry() registry.Registrar {
	return Lynx().ControlPlane().NewServiceRegistry()
}

func ServiceDiscovery() registry.Discovery {
	return Lynx().ControlPlane().NewServiceDiscovery()
}
