package app

import (
	"github.com/go-lynx/lynx/app/conf"
	"github.com/go-lynx/lynx/plugin"
)

var (
	// The app is in Singleton pattern
	app *LynxApp
)

type LynxApp struct {
	host        string
	name        string
	version     string
	plugManager LynxPluginManager
}

func App() *LynxApp {
	return app
}

func Host() string {
	return app.host
}

func Name() string {
	return app.name
}

func version() string {
	return app.version
}

// NewApp create a lynx microservice
func NewApp(lynx *conf.Lynx, p ...plugin.Plugin) *LynxApp {
	a := &LynxApp{
		name:    lynx.Application.Name,
		version: lynx.Application.Version,
	}

	// Manually load the plugins
	if p != nil && len(p) > 0 {
		a.plugManager.Init(p...)
	}

	// The app is in Singleton pattern
	app = a
	return a
}

func (a *LynxApp) PlugManager() *LynxPluginManager {
	return &a.plugManager
}
