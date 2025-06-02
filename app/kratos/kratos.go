// Package kratos provides integration with the Kratos framework
package kratos

import (
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-lynx/lynx/app/log"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/go-lynx/lynx/app"
)

// Options holds the configuration for creating a Kratos application
// Options 结构体用于存储创建 Kratos 应用所需的配置信息
type Options struct {
	// GRPCServer instance
	// gRPC 服务器实例
	GRPCServer *grpc.Server
	// HTTPServer instance
	// HTTP 服务器实例
	HTTPServer *http.Server
	// Registrar for service registration
	// 用于服务注册的注册器
	Registrar registry.Registrar
}

// NewKratos creates a new Kratos application with the specified options.
// It supports creating applications with HTTP server, gRPC server, or both.
//
// Parameters:
//   - opts: Options struct containing all necessary configuration
//
// Returns:
//   - *kratos.App: The created Kratos application
//   - error: Any error that occurred during creation
//
// NewKratos 使用指定的选项创建一个新的 Kratos 应用程序。
// 支持创建包含 HTTP 服务器、gRPC 服务器或两者兼有的应用程序。
//
// 参数:
//   - opts: 包含所有必要配置的 Options 结构体
//
// 返回值:
//   - *kratos.App: 创建好的 Kratos 应用程序实例
//   - error: 创建过程中发生的任何错误
func NewKratos(opts Options) (*kratos.App, error) {
	// Prepare base options for Kratos application
	// 为 Kratos 应用程序准备基础选项
	kratosOpts := []kratos.Option{
		// Set the application ID to the host name
		// 将应用程序 ID 设置为主机名
		kratos.ID(app.GetHost()),
		// Set the application name
		// 设置应用程序名称
		kratos.Name(app.GetName()),
		// Set the application version
		// 设置应用程序版本
		kratos.Version(app.GetVersion()),
		// Set the application metadata
		// 设置应用程序元数据
		kratos.Metadata(map[string]string{}),
		// Set the application logger
		// 设置应用程序日志记录器
		kratos.Logger(log.Logger),
		// Set the application registrar
		// 设置应用程序注册器
		kratos.Registrar(opts.Registrar),
	}

	// Collect all available transport servers based on the provided options.
	// 根据提供的选项收集所有可用的传输服务器。
	// This function checks for the presence of both gRPC and HTTP servers
	// in the options and adds them to a list of transport servers.
	// 此函数会检查选项中是否存在 gRPC 和 HTTP 服务器，
	// 并将它们添加到传输服务器列表中。
	// If any servers are available, it appends them to the Kratos application options.
	// 如果有可用的服务器，会将它们添加到 Kratos 应用程序选项中。
	//
	// Variables:
	//   - serverList: A slice to hold all available transport servers.
	// 变量:
	//   - serverList: 用于存储所有可用传输服务器的切片。
	var serverList []transport.Server

	// Check if a gRPC server instance is provided in the options.
	// 检查选项中是否提供了 gRPC 服务器实例。
	// If available, add it to the list of transport servers.
	// 如果可用，将其添加到传输服务器列表中。
	if opts.GRPCServer != nil {
		// Add the gRPC server to the list of transport servers
		// 将 gRPC 服务器添加到传输服务器列表中
		serverList = append(serverList, opts.GRPCServer)
	}

	// Check if an HTTP server instance is provided in the options.
	// 检查选项中是否提供了 HTTP 服务器实例。
	// If available, add it to the list of transport servers.
	// 如果可用，将其添加到传输服务器列表中。
	if opts.HTTPServer != nil {
		// Add the HTTP server to the list of transport servers
		// 将 HTTP 服务器添加到传输服务器列表中
		serverList = append(serverList, opts.HTTPServer)
	}

	// Check if there are any servers in the list.
	// 检查列表中是否有服务器。
	// If so, append them to the Kratos application options.
	// 如果有，将它们添加到 Kratos 应用程序选项中。
	if len(serverList) > 0 {
		// Add all collected servers to the Kratos application options
		// 将所有收集到的服务器添加到 Kratos 应用程序选项中
		kratosOpts = append(kratosOpts, kratos.Server(serverList...))
	}

	// Create and return the Kratos application
	// 创建并返回 Kratos 应用程序
	return kratos.New(kratosOpts...), nil
}

// ProvideKratosOptions 根据传入的参数创建并返回一个 Options 结构体实例。
// 该结构体用于存储创建 Kratos 应用所需的配置信息。
//
// 参数:
//   - logger: 应用程序使用的日志记录器。
//   - grpcServer: gRPC 服务器实例。
//   - httpServer: HTTP 服务器实例。
//   - registrar: 用于服务注册的注册器。
//
// 返回值:
//   - Options: 包含所有传入配置信息的 Options 结构体实例。
func ProvideKratosOptions(
	grpcServer *grpc.Server,
	httpServer *http.Server,
	registrar registry.Registrar,
) Options {
	// 返回一个初始化后的 Options 结构体实例，将传入的参数赋值给对应的字段
	return Options{
		GRPCServer: grpcServer,
		HTTPServer: httpServer,
		Registrar:  registrar,
	}
}
