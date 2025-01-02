package seata

import (
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/internal/seata/conf"
	"github.com/seata/seata-go/pkg/client"
)

var (
	name       = "seata"
	confPrefix = "lynx.seata"
)

type Option func(g *SeataClient)

func Weight(w int) Option {
	return func(g *SeataClient) {
		g.weight = w
	}
}

func Config(c *conf.Seata) Option {
	return func(g *SeataClient) {
		g.conf = c
	}
}

type SeataClient struct {
	conf   *conf.Seata
	weight int
}

// Load 方法用于加载并初始化 Seata 插件
func (s *SeataClient) Load(b config.Value) (plugins.Plugin, error) {
	// 从配置值 b 中扫描并解析 Seata 插件的配置到 s.conf 中
	err := b.Scan(s.conf)
	// 如果发生错误，返回 nil 和错误信息
	if err != nil {
		return nil, err
	}
	// 使用 Lynx 应用的 Helper 记录 Seata 插件初始化的信息
	app.Lynx().Helper().Infof("Initializing Seata")
	// 如果 Seata 插件已启用，则初始化 Seata 客户端
	if s.conf.GetEnabled() {
		// 调用 client.InitPath 方法初始化 Seata 客户端，使用配置中的路径
		client.InitPath(s.conf.GetConfigPath())
	}
	// 使用 Lynx 应用的 Helper 记录 Seata 服务初始化成功的信息
	app.Lynx().Helper().Infof("Seata successfully initialized")
	// 返回 Seata 插件实例和 nil 错误，表示加载成功
	return s, nil
}

func (s *SeataClient) Unload() error {
	return nil
}

func Seata(opts ...Option) plugins.Plugin {
	s := &SeataClient{
		weight: 500,
		conf:   &conf.Seata{},
	}
	for _, option := range opts {
		option(s)
	}
	return s
}
