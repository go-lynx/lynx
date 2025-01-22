package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/internal/polaris/conf"
	"github.com/polarismesh/polaris-go/api"
)

var (
	name       = "polaris"
	confPrefix = "lynx.polaris"
)

type PlugPolaris struct {
	polaris *polaris.Polaris
	conf    *conf.Polaris
	weight  int
}

type Option func(h *PlugPolaris)

func Weight(w int) Option {
	return func(h *PlugPolaris) {
		h.weight = w
	}
}

func Config(c *conf.Polaris) Option {
	return func(p *PlugPolaris) {
		p.conf = c
	}
}

func (p *PlugPolaris) Load(b config.Value) (plugins.Plugin, error) {
	// 从配置值 b 中扫描并解析 Polaris 插件的配置到 p.conf 中。
	err := b.Scan(p.conf)
	// 如果发生错误，返回 nil 和错误信息。
	if err != nil {
		return nil, err
	}

	// 使用 Lynx 应用的 Helper 记录 Polaris 插件初始化的信息。
	app.Lynx().GetLogHelper().Infof("Initializing Polaris plugin")

	// 初始化 Polaris SDK 上下文。
	sdk, err := api.InitContextByConfig(api.NewConfiguration())
	// 如果初始化失败，记录错误信息并抛出 panic。
	if err != nil {
		app.Lynx().GetLogHelper().Error(err)
		panic(err)
	}

	// 创建一个新的 Polaris 实例，使用之前初始化的 SDK 和配置。
	polar := polaris.New(
		sdk,
		polaris.WithService(app.GetName()),
		polaris.WithNamespace(p.conf.Namespace),
	)
	// 将 Polaris 实例保存到 p.polaris 中。
	p.polaris = &polar

	// 设置 Polaris 控制平面为 Lynx 应用的控制平面。
	err = app.Lynx().SetControlPlane(p)
	if err != nil {
		app.Lynx().GetLogHelper().Error(err)
		return nil, err
	}

	// 获取 Lynx 应用的控制平面启动配置。
	cfg, err := app.Lynx().InitControlPlaneConfig()
	if err != nil {
		app.Lynx().GetLogHelper().Error(err)
		return nil, err
	}

	// 加载插件列表中的插件。
	app.Lynx().GetPluginManager().LoadPlugins(cfg)
	// 返回 Polaris 插件实例和 nil 错误，表示加载成功。
	return p, nil
}

func (p *PlugPolaris) Unload() error {
	return nil
}
