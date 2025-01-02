package boot

import (
	"flag"
	"fmt"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/encoding/json"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins"
	"google.golang.org/protobuf/encoding/protojson"
	"time"
)

var (
	flagConf string
)

type Boot struct {
	wire    wireApp
	plugins []plugins.Plugin
	conf    config.Config
}

func init() {
	flag.StringVar(&flagConf, "conf", "../../configs", "config path, eg: -conf config.yaml")
	flag.Parse()
	json.MarshalOptions = protojson.MarshalOptions{
		EmitUnpopulated: true,
		UseProtoNames:   true,
	}
}

type wireApp func(logger log.Logger) (*kratos.App, error)

// Run 方法是应用程序的启动入口点
func (b *Boot) Run() {
	// 延迟调用 handlePanic 方法，用于处理可能发生的 panic
	defer b.handlePanic()
	// 记录当前时间，用于计算启动耗时
	st := time.Now()

	// 加载本地启动配置文件
	b.loadLocalBootFile()
	// 创建一个新的 Lynx 应用实例，传入配置和插件
	app.NewApp(b.conf, b.plugins...)
	// 初始化 Lynx 应用的日志记录器
	app.Lynx().InitLogger()
	// 记录一条信息，指示 Lynx 应用正在启动
	app.Lynx().Helper().Infof("Lynx application is starting up")
	// 准备插件，可能包括加载配置等操作
	app.Lynx().PlugManager().PreparePlug(b.conf)

	// 先加载插件，然后执行 wireApp
	app.Lynx().PlugManager().LoadPlugins(b.conf)
	// 调用 wireApp 函数，创建并返回一个 Kratos 应用实例和错误信息
	k, err := b.wire(app.Lynx().Logger())
	// 如果发生错误，记录错误信息并抛出 panic
	if err != nil {
		app.Lynx().Helper().Error(err)
		panic(err)
	}

	// 计算启动耗时（毫秒）
	t := (time.Now().UnixNano() - st.UnixNano()) / 1e6
	// 记录一条信息，指示 Lynx 应用启动成功，并显示启动耗时
	app.Lynx().Helper().Infof("Lynx application started successfully，elapsed time：%v ms, port listening initiated.", t)

	// 启动 Kratos 应用，并等待停止信号
	if err := k.Run(); err != nil {
		// 如果发生错误，记录错误信息并抛出 panic
		app.Lynx().Helper().Error(err)
		panic(err)
	}
}

// handlePanic 方法用于处理应用程序运行过程中可能发生的 panic
func (b *Boot) handlePanic() {
	// 捕获 recover() 函数返回的 panic 信息
	if r := recover(); r != nil {
		// 将 recover() 返回的结果转换为 error 类型
		err, ok := r.(error)
		// 如果转换失败，则将 recover() 返回的结果转换为字符串，并包装成 error 类型
		if !ok {
			err = fmt.Errorf("%v", r)
		}

		// 如果 Lynx 助手已经初始化，则使用它来记录错误，否则使用标准日志包
		if helper := app.Lynx().Helper(); helper != nil {
			helper.Error(err)
		} else {
			log.Error(err)
		}
	}

	// 无论是否发生 panic，都卸载插件
	if app.Lynx() != nil && app.Lynx().PlugManager() != nil {
		app.Lynx().PlugManager().UnloadPlugins()
	}
}

// LynxApplication Create a Lynx microservice bootstrap program
func LynxApplication(wire wireApp, p ...plugins.Plugin) *Boot {
	return &Boot{
		wire:    wire,
		plugins: p,
	}
}
