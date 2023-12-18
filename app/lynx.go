package app

import (
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/app/tx"
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
	cert         Cert
	logger       log.Logger
	globalConf   config.Config
	controlPlane ControlPlane
	plugManager  LynxPluginManager
	txManager    tx.TransactionManager

	dfLog *log.Helper
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
func NewApp(c config.Config, p ...plugin.Plugin) *LynxApp {
	host, _ := os.Hostname()
	var bootConf conf.Bootstrap
	err := c.Scan(&bootConf)
	if err != nil {
		return nil
	}

	var app = &LynxApp{
		host:         host,
		name:         bootConf.Lynx.Application.Name,
		version:      bootConf.Lynx.Application.Version,
		globalConf:   c,
		plugManager:  NewLynxPluginManager(p...),
		txManager:    tx.NewLynxTransactionManager(),
		controlPlane: &LocalControlPlane{},
	}
	// The lynxApp is in Singleton pattern
	lynxApp = app
	return app
}

func (a *LynxApp) PlugManager() LynxPluginManager {
	return a.plugManager
}

func (a *LynxApp) GlobalConfig() config.Config {
	return a.globalConf
}

func (a *LynxApp) setGlobalConfig(c config.Config) {
	// Close the last configuration
	if a.globalConf != nil {
		err := a.globalConf.Close()
		if err != nil {
			a.Helper().Error(err.Error())
		}
	}
	a.globalConf = c
}

func (a *LynxApp) TxManager() tx.TransactionManager {
	return a.txManager
}
