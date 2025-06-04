package seata

import (
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/seata/conf"
	"github.com/seata/seata-go/pkg/client"
)

// Plugin metadata
// 插件元数据，定义插件的基本信息
const (
	// pluginName 是 HTTP 服务器插件的唯一标识符，用于在插件系统中识别该插件。
	pluginName = "seata.server"

	// pluginVersion 表示 HTTP 服务器插件的当前版本。
	pluginVersion = "v2.0.0"

	// pluginDescription 简要描述了 HTTP 服务器插件的功能。
	pluginDescription = "seata transaction server plugin for Lynx framework"

	// confPrefix 是加载 HTTP 服务器配置时使用的配置前缀。
	confPrefix = "lynx.seata"
)

type TxSeataClient struct {
	// 嵌入基础插件，继承插件的通用属性和方法
	*plugins.BasePlugin
	// HTTP 服务器的配置信息
	conf *conf.Seata
}

// NewSeataClient 创建一个新的 HTTP 服务器插件实例。
// 该函数初始化插件的基础信息，并返回一个指向 ServiceHttp 结构体的指针。
func NewSeataClient() *TxSeataClient {
	return &TxSeataClient{
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
			90,
		),
	}
}

// InitializeResources 方法用于加载并初始化 Seata 插件
func (t *TxSeataClient) InitializeResources(rt plugins.Runtime) error {
	// 从配置值 b 中扫描并解析 Seata 插件的配置到 t.conf 中
	err := rt.GetConfig().Scan(t.conf)
	// 如果发生错误，返回 nil 和错误信息
	if err != nil {
		return err
	}
	return nil
}

func (t *TxSeataClient) StartupTasks() error {
	// 使用 Lynx 应用的 Helper 记录 Seata 插件初始化的信息
	log.Infof("Initializing seata")
	// 如果 Seata 插件已启用，则初始化 Seata 客户端
	if t.conf.GetEnabled() {
		// 调用 client.InitPath 方法初始化 Seata 客户端，使用配置中的路径
		client.InitPath(t.conf.GetConfigFilePath())
	}
	// 使用 Lynx 应用的 Helper 记录 Seata 服务初始化成功的信息
	log.Infof("seata successfully initialized")
	// 返回 Seata 插件实例和 nil 错误，表示加载成功
	return nil
}

func (t *TxSeataClient) CleanupTasks() error {
	return nil
}
