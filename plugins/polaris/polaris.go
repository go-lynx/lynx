package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/plugins/polaris/v2/conf"
	"github.com/polarismesh/polaris-go/api"
	"google.golang.org/protobuf/types/known/durationpb"
	"math"
)

// Plugin metadata
// 插件元数据，定义插件的基本信息
const (
	// pluginName 是 HTTP 服务器插件的唯一标识符，用于在插件系统中识别该插件。
	pluginName = "polaris.control.plane"

	// pluginVersion 表示 HTTP 服务器插件的当前版本。
	pluginVersion = "v2.0.0"

	// pluginDescription 简要描述了 HTTP 服务器插件的功能。
	pluginDescription = "polaris control plane plugin for lynx framework"

	// confPrefix 是加载 HTTP 服务器配置时使用的配置前缀。
	confPrefix = "lynx.polaris"
)

type PlugPolaris struct {
	*plugins.BasePlugin
	polaris *polaris.Polaris
	conf    *conf.Polaris
}

// NewPolarisControlPlane 创建一个新的 控制平面 Polaris。
// 该函数初始化插件的基础信息，并返回一个指向 PolarisControlPlane 的指针。
func NewPolarisControlPlane() *PlugPolaris {
	return &PlugPolaris{
		BasePlugin: plugins.NewBasePlugin(
			// 生成插件的唯一 ID
			plugins.GeneratePluginID("", pluginName, pluginVersion),
			// 插件名称
			pluginName,
			// 插件描述
			pluginDescription,
			// 插件版本
			pluginVersion,
			// 配置前缀
			confPrefix,
			// 权重
			math.MaxInt,
		),
	}
}

// InitializeResources 实现了 Polaris 插件的自定义初始化逻辑。
// 该函数会加载并验证 Polaris 配置，如果配置未提供，则使用默认配置。
func (p *PlugPolaris) InitializeResources(rt plugins.Runtime) error {
	// 初始化一个空的配置结构
	p.conf = &conf.Polaris{}

	// 从运行时配置中扫描并加载 Polaris 配置
	err := rt.GetConfig().Value(confPrefix).Scan(p.conf)
	if err != nil {
		return err
	}

	// 设置默认配置
	defaultConf := &conf.Polaris{
		// 默认命名空间为 default
		Namespace: "default",
		// 默认服务实例权重为 100
		Weight: 100,
		// 默认 TTL 为 5 秒
		Ttl: 5,
		// 默认超时时间为 5 秒
		Timeout: &durationpb.Duration{Seconds: 5},
	}

	// 对未设置的字段使用默认值
	if p.conf.Namespace == "" {
		p.conf.Namespace = defaultConf.Namespace
	}
	if p.conf.Weight == 0 {
		p.conf.Weight = defaultConf.Weight
	}
	if p.conf.Ttl == 0 {
		p.conf.Ttl = defaultConf.Ttl
	}
	if p.conf.Timeout == nil {
		p.conf.Timeout = defaultConf.Timeout
	}

	return nil
}

// StartupTasks 实现了 HTTP 插件的自定义启动逻辑。
// 该函数会配置并启动 HTTP 服务器，添加必要的中间件和配置选项。
func (p *PlugPolaris) StartupTasks() error {
	// 使用 Lynx 应用的 Helper 记录 Polaris 插件初始化的信息。
	log.Infof("Initializing polaris plugin")

	// 初始化 Polaris SDK 上下文。
	sdk, err := api.InitContextByConfig(api.NewConfiguration())
	// 如果初始化失败，记录错误信息并抛出 panic。
	if err != nil {
		log.Error(err)
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
		log.Error(err)
		return err
	}

	// 获取 Lynx 应用的控制平面启动配置。
	cfg, err := app.Lynx().InitControlPlaneConfig()
	if err != nil {
		log.Error(err)
		return err
	}

	// 加载插件列表中的插件。
	app.Lynx().GetPluginManager().LoadPlugins(cfg)
	return nil
}
