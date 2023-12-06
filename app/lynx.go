package app

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/conf"
	"github.com/go-lynx/lynx/plugin"
	"os"
)

var (
	// The lynxApp is in Singleton pattern
	lynxApp *LynxApp
)

type LynxApp struct {
	host         string
	name         string
	version      string
	conf         *conf.Lynx
	dfLog        *log.Helper
	logger       log.Logger
	tls          *conf.Tls
	controlPlane ControlPlane
	plugManager  *LynxPluginManager
}

func Lynx() *LynxApp {
	return lynxApp
}

func Host() string {
	return lynxApp.host
}

func Name() string {
	return lynxApp.name
}

func Version() string {
	return lynxApp.version
}

// NewApp create a lynx microservice
func NewApp(c *conf.Bootstrap, p ...plugin.Plugin) *LynxApp {
	host, _ := os.Hostname()
	var app = &LynxApp{
		host:        host,
		name:        c.Lynx.Application.Name,
		version:     c.Lynx.Application.Version,
		conf:        c.Lynx,
		plugManager: NewLynxPluginManager(p...),
	}
	// The lynxApp is in Singleton pattern
	lynxApp = app
	return app
}

func (a *LynxApp) PlugManager() *LynxPluginManager {
	return a.plugManager
}

func (a *LynxApp) GetConfig() *conf.Lynx {
	return a.conf
}
