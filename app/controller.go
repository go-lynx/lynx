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

type LocalControlPlane struct {
}

func (c *LocalControlPlane) HttpRateLimit() middleware.Middleware {
	return nil
}

func (c *LocalControlPlane) GrpcRateLimit() middleware.Middleware {
	return nil
}

func (c *LocalControlPlane) NewServiceRegistry() registry.Registrar {
	return nil
}

func (c *LocalControlPlane) NewServiceDiscovery() registry.Discovery {
	return nil
}

func (c *LocalControlPlane) NewNodeRouter(name string) selector.NodeFilter {
	return nil
}

func (c *LocalControlPlane) Config(fileName string, group string) (config.Source, error) {
	return nil, nil
}

func (c *LocalControlPlane) Namespace() string {
	return ""
}

func (a *LynxApp) ControlPlane() ControlPlane {
	return Lynx().controlPlane
}

func (a *LynxApp) SetControlPlane(plane ControlPlane) {
	Lynx().controlPlane = plane
}

func (a *LynxApp) ControlPlaneBootConfiguration() config.Config {
	// 检查控制平面是否已初始化，如果没有，则创建一个新的配置对象
	if Lynx().ControlPlane() == nil {
		return config.New()
	}

	// 默认情况下，加载应用程序名称 + .yaml 格式的文件
	yaml := Name() + ".yaml"

	// 记录日志，指示正在从配置中心读取文件，包括文件名、组名和命名空间
	Lynx().Helper().Infof("Reading from the configuration center,file:[%v] group:[%v] namespace:[%v]", yaml, Name(), Lynx().ControlPlane().Namespace())

	// 尝试从控制平面获取配置源，文件名是 yaml，组名是应用程序名称
	s, err := Lynx().ControlPlane().Config(yaml, Name())
	// 如果获取配置源时发生错误，记录错误并抛出 panic
	if err != nil {
		Lynx().Helper().Errorf("Failed to read the configuration file:[%v] group:[%v] namespace:[%v]", yaml, Name(), Lynx().ControlPlane().Namespace())
		panic(err)
	}

	// 创建一个新的配置对象，使用获取到的配置源
	c := config.New(config.WithSource(s))
	// 加载配置，如果加载过程中发生错误，抛出 panic
	if err := c.Load(); err != nil {
		panic(err)
	}

	// 将全局配置设置为加载的配置
	a.setGlobalConfig(c)
	// 返回加载的配置对象
	return c
}

func ServiceRegistry() registry.Registrar {
	return Lynx().ControlPlane().NewServiceRegistry()
}

func ServiceDiscovery() registry.Discovery {
	return Lynx().ControlPlane().NewServiceDiscovery()
}
