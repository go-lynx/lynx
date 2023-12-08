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
	log.Infof("Lynx Log component loading")
	a.logger = log.With(log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.id", Host(),
		"service.name", Name(),
		"service.version", Version(),
		"trace.id", tracing.TraceID,
		"span.id", tracing.SpanID,
	)
	a.dfLog = log.NewHelper(a.logger)
	log.Info("Lynx Log component loaded successfully")
	data, err := fs.ReadFile(configFS, "banner.txt")
	if err != nil {
		log.Fatal(err)
	}

	var boot conf.Bootstrap
	err = a.GetGlobalConfig().Scan(&boot)
	if err != nil {
		panic(err)
	}

	if !boot.GetLynx().GetApplication().GetCloseBanner() {
		a.GetHelper().Infof("\n" + string(data))
	}
}

func (a *LynxApp) GetHelper() *log.Helper {
	return a.dfLog
}

func (a *LynxApp) GetLogger() log.Logger {
	return a.logger
}
