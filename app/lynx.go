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
	host        string
	name        string
	version     string
	dfLog       *log.Helper
	logger      log.Logger
	tls         *conf.Tls
	plugManager *LynxPluginManager
	cp          ControlPlane
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
		plugManager: NewLynxPluginManager(p...),
	}
	// The lynxApp is in Singleton pattern
	lynxApp = app
	return app
}

func (a *LynxApp) PlugManager() *LynxPluginManager {
	return a.plugManager
}
