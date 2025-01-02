package app

import (
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/conf"
	"github.com/go-lynx/lynx/plugins"
	"os"
)

var (
	// The lynxApp is in Singleton pattern
	lynxApp *LynxApp
)

type LynxApp struct {
	host          string
	name          string
	version       string
	cert          Cert
	logger        log.Logger
	globalConf    config.Config
	controlPlane  ControlPlane
	pluginManager LynxPluginManager

	dfLog *log.Helper
}

// Lynx function returns a global LynxApp instance
func Lynx() *LynxApp {
	return lynxApp
}

// Host Retrieves the host name of the current application instance
func Host() string {
	// Returns the host name stored in the lynxApp instance
	return lynxApp.host
}

func Name() string {
	return lynxApp.name
}

func Version() string {
	return lynxApp.version
}

// NewApp 函数用于创建一个新的 Lynx 应用实例
func NewApp(c config.Config, p ...plugins.Plugin) *LynxApp {
	// 获取当前主机名
	host, _ := os.Hostname()

	// 定义一个 Bootstrap 配置对象，用于存储应用的启动配置
	var bootConf conf.Bootstrap

	// 从全局配置对象 c 中扫描并解析出 Bootstrap 配置到 bootConf 中
	err := c.Scan(&bootConf)
	// 如果发生错误，返回 nil
	if err != nil {
		return nil
	}

	// 创建一个新的 LynxApp 实例
	var app = &LynxApp{
		// 设置主机名为当前主机名
		host: host,
		// 设置应用名为 Bootstrap 配置中的应用名
		name: bootConf.Lynx.Application.Name,
		// 设置应用版本为 Bootstrap 配置中的应用版本
		version: bootConf.Lynx.Application.Version,
		// 设置全局配置对象
		globalConf: c,
		// 创建一个新的 LynxPluginManager 实例，并传入插件列表
		pluginManager: NewLynxPluginManager(p...),
		// 设置控制平面为本地控制平面实例
		controlPlane: &LocalControlPlane{},
	}

	// 将新创建的 LynxApp 实例设置为全局单例
	lynxApp = app

	// 返回新创建的 LynxApp 实例
	return app
}

func (a *LynxApp) PlugManager() LynxPluginManager {
	return a.pluginManager
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
