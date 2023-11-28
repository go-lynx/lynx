package app

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"os"
)

var (
	// dfLog is log manage module
	dfLog  *log.Helper
	logger *log.Logger
)

func InitLogger() log.Logger {
	log.Infof("Lynx Log component loading")
	l := log.With(log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.id", app.host,
		"service.name", app.name,
		"service.version", app.version,
		"trace.id", tracing.TraceID,
		"span.id", tracing.SpanID,
	)
	dfLog = log.NewHelper(l)
	log.Info("Lynx Log component loaded successfully")
	logger = &l
	return *logger
}

func GetHelper() *log.Helper {
	return dfLog
}

func GetLogger() log.Logger {
	return *logger
}
