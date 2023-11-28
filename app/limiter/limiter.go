package limiter

import (
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-lynx/lynx/app/conf"
)

type Limiter interface {
	HttpRateLimit(lynx *conf.Lynx) middleware.Middleware
	GrpcRateLimit(lynx *conf.Lynx) middleware.Middleware
}
