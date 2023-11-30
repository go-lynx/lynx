package app

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"os"
)

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
}

func (a *LynxApp) GetHelper() *log.Helper {
	return Lynx().dfLog
}

func (a *LynxApp) GetLogger() log.Logger {
	return Lynx().logger
}
