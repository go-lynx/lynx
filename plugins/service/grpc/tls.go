package grpc

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/app"
)

// tlsLoad creates and configures TLS settings for the gRPC server.
// tlsLoad 为 gRPC 服务器创建并配置 TLS 设置。
// It performs the following operations:
// 它执行以下操作：
//   - Loads the X.509 certificate and private key pair
//   - 加载 X.509 证书和私钥对
//   - Creates a certificate pool and adds the root CA certificate
//   - 创建证书池并添加根 CA 证书
//   - Configures TLS settings including client authentication type
//   - 配置 TLS 设置，包括客户端认证类型
//
// The method will panic if:
// 该方法在以下情况下会发生 panic：
//   - The certificate and key pair cannot be loaded
//   - 无法加载证书和私钥对
//   - The root CA certificate cannot be added to the certificate pool
//   - 无法将根 CA 证书添加到证书池中
//
// Returns:
// 返回：
//   - grpc.ServerOption: A configured TLS option for the gRPC server
//   - grpc.ServerOption: 为 gRPC 服务器配置好的 TLS 选项
func (g *ServiceGrpc) tlsLoad() grpc.ServerOption {
	// Load the X.509 certificate and private key pair from the paths provided by the application.
	// 从应用程序提供的路径加载 X.509 证书和私钥对。
	// app.Lynx().Cert().GetCrt() returns the path to the certificate file.
	// app.Lynx().Cert().GetCrt() 返回证书文件的路径。
	// app.Lynx().Cert().GetKey() returns the path to the private key file.
	// app.Lynx().Cert().GetKey() 返回私钥文件的路径。
	tlsCert, err := tls.X509KeyPair(app.Lynx().Cert().GetCrt(), app.Lynx().Cert().GetKey())
	if err != nil {
		// If there is an error loading the certificate and key pair, panic with the error.
		// 如果加载证书和私钥对时出错，则使用该错误触发 panic。
		panic(err)
	}

	// Create a new certificate pool to hold trusted root CA certificates.
	// 创建一个新的证书池，用于存放受信任的根 CA 证书。
	certPool := x509.NewCertPool()
	// Attempt to add the root CA certificate (in PEM format) to the certificate pool.
	// 尝试将根 CA 证书（PEM 格式）添加到证书池中。
	// app.Lynx().Cert().GetRootCA() returns the PEM-encoded root CA certificate.
	// app.Lynx().Cert().GetRootCA() 返回 PEM 编码的根 CA 证书。
	if !certPool.AppendCertsFromPEM(app.Lynx().Cert().GetRootCA()) {
		// If adding the root CA certificate fails, panic with the error.
		// 如果添加根 CA 证书失败，则使用该错误触发 panic。
		panic(err)
	}

	// Configure the TLS settings for the gRPC server.
	// 为 gRPC 服务器配置 TLS 设置。
	// Certificates: Set the server's certificate and private key pair.
	// Certificates: 设置服务器的证书和私钥对。
	// ClientCAs: Set the certificate pool containing trusted root CA certificates for client authentication.
	// ClientCAs: 设置包含受信任根 CA 证书的证书池，用于客户端认证。
	// ServerName: Set the server name, which is retrieved from the application configuration.
	// ServerName: 设置服务器名称，从应用程序配置中获取。
	// ClientAuth: Set the client authentication type based on the configuration.
	// ClientAuth: 根据配置设置客户端认证类型。
	return grpc.TLSConfig(&tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		ClientCAs:    certPool,
		ServerName:   app.GetName(),
		ClientAuth:   tls.ClientAuthType(g.conf.GetTlsAuthType()),
	})
}
