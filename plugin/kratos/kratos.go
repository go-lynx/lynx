package kratos

import (
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/go-lynx/lynx/app"
)

// NewKratos Start kratos application
func NewKratos(logger log.Logger, gs *grpc.Server, hs *http.Server, r registry.Registrar) *kratos.App {
	return kratos.New(
		kratos.ID(app.Host()),
		kratos.Name(app.Name()),
		kratos.Version(app.Version()),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			gs,
			hs,
		),
		kratos.Registrar(r),
	)
}

// NewGrpcKratos Start kratos application
func NewGrpcKratos(logger log.Logger, gs *grpc.Server, r registry.Registrar) *kratos.App {
	return kratos.New(
		kratos.ID(app.Host()),
		kratos.Name(app.Name()),
		kratos.Version(app.Version()),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			gs,
		),
		kratos.Registrar(r),
	)
}

// NewHttpKratos Start kratos application
func NewHttpKratos(logger log.Logger, hs *http.Server, r registry.Registrar) *kratos.App {
	return kratos.New(
		kratos.ID(app.Host()),
		kratos.Name(app.Name()),
		kratos.Version(app.Version()),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			hs,
		),
		kratos.Registrar(r),
	)
}
