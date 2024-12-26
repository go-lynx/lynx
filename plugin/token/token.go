package token

import (
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugin"
	"github.com/go-lynx/lynx/plugin/token/conf"
)

var (
	name       = "token"
	confPrefix = "lynx.token"
)

type PlugToken struct {
	load   []LoaderToken
	conf   *conf.Token
	weight int
}

// Load 方法用于加载并初始化 Token 插件
func (t *PlugToken) Load(b config.Value) (plugin.Plugin, error) {
	// 从配置值 b 中扫描并解析 Token 插件的配置到 t.conf 中
	err := b.Scan(t.conf)
	// 如果发生错误，返回 nil 和错误信息
	if err != nil {
		return nil, err
	}

	// 使用 Lynx 应用的 Helper 记录 Token 插件初始化的信息
	app.Lynx().Helper().Infof("Initializing service load")
	// 从 Lynx 应用的控制平面获取配置源，使用配置中的文件名和组名
	source, err := app.Lynx().ControlPlane().Config(t.conf.GetFileName(), t.conf.GetGroup())
	// 如果发生错误，返回 nil 和错误信息
	if err != nil {
		return nil, err
	}

	// 创建一个新的配置对象，使用获取的配置源
	c := config.New(config.WithSource(source))
	// 加载配置
	if err := c.Load(); err != nil {
		return nil, err
	}

	// 如果有加载器并且加载器数量大于 0，则遍历加载器列表
	if t.load != nil && len(t.load) > 0 {
		for _, l := range t.load {
			// 初始化每个加载器
			err := l.Init(c)
			// 如果发生错误，返回 nil 和错误信息
			if err != nil {
				return nil, err
			}
		}
	}
	// 使用 Lynx 应用的 Helper 记录 Token 服务初始化成功的信息
	app.Lynx().Helper().Infof("Service load successfully initialized")
	// 返回 Token 插件实例和 nil 错误，表示加载成功
	return t, nil
}

func (t *PlugToken) Unload() error {
	return nil
}

func Token(token ...LoaderToken) plugin.Plugin {
	return &PlugToken{
		weight: 0,
		load:   token,
		conf:   &conf.Token{},
	}
}

type LoaderToken interface {
	Init(conf config.Config) error
}
