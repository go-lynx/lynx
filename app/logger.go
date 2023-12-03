package app

import (
	"embed"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"io/fs"
	"os"
)

//go:embed banner.txt
var configFS embed.FS

func (a *LynxApp) InitLogger() {
	log.Infof("Lynx Log component loading")
	a.logger = log.With(log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.id", lynxApp.host,
		"service.name", lynxApp.name,
		"service.version", lynxApp.version,
		"trace.id", tracing.TraceID,
		"span.id", tracing.SpanID,
	)
	a.dfLog = log.NewHelper(a.logger)
	log.Info("Lynx Log component loaded successfully")
	data, err := fs.ReadFile(configFS, "banner.txt")
	if err != nil {
		log.Fatal(err)
	}
	a.GetHelper().Infof("\n" + string(data))
}

func (a *LynxApp) GetHelper() *log.Helper {
	return Lynx().dfLog
}

func (a *LynxApp) GetLogger() log.Logger {
	return Lynx().logger
}
