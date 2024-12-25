package app

import (
	"embed"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-lynx/lynx/conf"
	"io/fs"
	"os"
)

//go:embed banner.txt
var configFS embed.FS

func (a *LynxApp) InitLogger() {
	// 打印日志，指示 Lynx 日志组件正在加载
	log.Infof("Lynx Log component loading")

	// 初始化日志记录器，使用标准输出作为日志输出，添加默认的时间戳、调用者信息、服务 ID、服务名称、服务版本、跟踪 ID 和跨度 ID
	a.logger = log.With(
		log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.id", Host(),
		"service.name", Name(),
		"service.version", Version(),
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)

	// 初始化日志助手，用于更方便地记录日志
	a.dfLog = log.NewHelper(a.logger)

	// 打印日志，指示 Lynx 日志组件加载成功
	log.Info("Lynx Log component loaded successfully")

	// 从嵌入的文件系统中读取 banner.txt 文件内容
	data, err := fs.ReadFile(configFS, "banner.txt")
	// 如果读取文件时发生错误，记录错误并使用 log.Fatal 终止程序
	if err != nil {
		log.Fatal(err)
	}

	// 初始化一个 Bootstrap 结构体，用于存储应用程序的配置信息
	var boot conf.Bootstrap
	// 从全局配置中扫描并填充 Bootstrap 结构体
	err = a.GlobalConfig().Scan(&boot)
	// 如果扫描过程中发生错误，抛出 panic
	if err != nil {
		panic(err)
	}

	// 如果应用程序配置中的 close_banner 字段为 false，则打印横幅信息
	if !boot.GetLynx().GetApplication().GetCloseBanner() {
		// 使用日志助手打印横幅信息
		a.Helper().Infof("\n" + string(data))
	}
}

func (a *LynxApp) Helper() *log.Helper {
	return a.dfLog
}

func (a *LynxApp) Logger() log.Logger {
	return a.logger
}
