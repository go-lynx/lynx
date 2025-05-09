package boot

import (
	"flag"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/encoding/json"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins"
	"google.golang.org/protobuf/encoding/protojson"
)

// flagConf 存储从命令行参数中获取的配置文件路径
var (
	// flagConf holds the configuration file path from command line arguments
	flagConf string
)

// Boot 表示 Lynx 应用程序的主要引导结构，负责管理应用的初始化、配置加载和生命周期
type Boot struct {
	wire    wireApp          // 用于初始化 Kratos 应用程序的函数
	plugins []plugins.Plugin // 要初始化的插件列表
	conf    config.Config    // 应用程序的配置实例
	cleanup func()           // 清理函数，用于在应用关闭时执行资源清理操作
}

// init 包初始化函数，用于解析命令行参数并配置 JSON 序列化选项
func init() {
	// 从命令行参数中获取配置文件路径，默认值为 "../../configs"
	flag.StringVar(&flagConf, "conf", "../../configs", "config path, eg: -conf config.yaml")
	flag.Parse()

	// 配置 protocol buffers 的 JSON 序列化选项
	// EmitUnpopulated: 序列化时包含未设置的字段
	// UseProtoNames: 使用 proto 文件中定义的字段名
	json.MarshalOptions = protojson.MarshalOptions{
		EmitUnpopulated: true,
		UseProtoNames:   true,
	}
}

// wireApp 是一个函数类型，用于初始化并返回一个 Kratos 应用程序实例
type wireApp func(logger log.Logger) (*kratos.App, error)

// Run 启动 Lynx 应用程序并管理其生命周期
func (b *Boot) Run() error {
	// 检查 Boot 实例是否为 nil
	if b == nil {
		return fmt.Errorf("boot instance is nil")
	}

	// 延迟执行 panic 处理和清理操作
	defer b.handlePanic()
	if b.cleanup != nil {
		defer b.cleanup()
	}

	// 记录应用启动时间，用于计算启动耗时
	startTime := time.Now()

	// 加载引导配置
	if err := b.LoadLocalBootstrapConfig(); err != nil {
		return fmt.Errorf("failed to load bootstrap configuration: %w", err)
	}

	// 初始化 Lynx 应用程序
	lynxApp, err := app.NewApp(b.conf, b.plugins...)
	if err != nil {
		return fmt.Errorf("failed to create Lynx application: %w", err)
	}

	// 初始化日志记录器
	if err := lynxApp.InitLogger(); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	// 获取日志助手
	helper := lynxApp.GetLogHelper()
	if helper == nil {
		return fmt.Errorf("log helper is nil")
	}

	// 记录应用启动信息
	helper.Info("Lynx application is starting up")

	// 获取插件管理器
	pluginManager := lynxApp.GetPluginManager()
	if pluginManager == nil {
		return fmt.Errorf("plugin manager is nil")
	}

	// 加载插件
	pluginManager.LoadPlugins(b.conf)

	// 初始化 Kratos 应用程序
	kratosApp, err := b.wire(lynxApp.GetLogger())
	if err != nil {
		helper.Error(err)
		return fmt.Errorf("failed to initialize Kratos application: %w", err)
	}

	// 计算应用启动耗时
	elapsedMs := time.Since(startTime).Milliseconds()
	helper.Infof("Lynx application started successfully, elapsed time: %d ms, port listening initiated", elapsedMs)

	// 运行 Kratos 应用程序
	if err := kratosApp.Run(); err != nil {
		helper.Error(err)
		return fmt.Errorf("failed to run Kratos application: %w", err)
	}

	return nil
}

// handlePanic 用于从 panic 中恢复，并确保资源的正确清理
func (b *Boot) handlePanic() {
	// 捕获 panic
	if r := recover(); r != nil {
		var err error
		// 根据 panic 的类型转换为 error
		switch v := r.(type) {
		case error:
			err = v
		case string:
			err = fmt.Errorf(v)
		default:
			err = fmt.Errorf("%v", r)
		}

		// 使用合适的日志记录器记录错误信息
		lynxApp := app.Lynx()
		if lynxApp != nil {
			if helper := lynxApp.GetLogHelper(); helper != nil {
				helper.Error(err)
			} else {
				log.Error(err)
			}
		} else {
			log.Error(err)
		}

		// 确保插件被卸载
		if lynxApp != nil && lynxApp.GetPluginManager() != nil {
			lynxApp.GetPluginManager().UnloadPlugins()
		}
	}
}

// NewLynxApplication 创建一个新的 Lynx 微服务引导程序实例
// 参数:
//   - wire: 用于初始化 Kratos 应用程序的函数
//   - plugins: 可选的插件列表，用于随应用一起初始化
//
// 返回值:
//   - *Boot: 初始化后的 Boot 实例
func NewLynxApplication(wire wireApp, plugins ...plugins.Plugin) *Boot {
	// 检查 wire 函数是否为 nil
	if wire == nil {
		log.Error("wire function cannot be nil")
		return nil
	}

	// 返回初始化后的 Boot 实例
	return &Boot{
		wire:    wire,
		plugins: plugins,
	}
}
