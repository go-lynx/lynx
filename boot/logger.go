package boot

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
	l := log.With(log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.id", id,
		"service.name", name,
		"service.version", version,
		"trace.id", tracing.TraceID,
		"span.id", tracing.SpanID,
	)
	dfLog = log.NewHelper(l)
	dfLog.Infof("Log component loaded successfully")
	logger = &l
	return *logger
}

func GetHelper() *log.Helper {
	return dfLog
}

func GetLogger() log.Logger {
	return *logger
}
