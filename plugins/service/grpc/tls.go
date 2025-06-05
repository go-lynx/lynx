package grpc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

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
	// Get the certificate provider
	certProvider := app.Lynx().Certificate()
	if certProvider == nil {
		panic("certificate provider not configured")
	}

	// Load certificate and private key
	tlsCert, err := tls.X509KeyPair(certProvider.GetCertificate(), certProvider.GetPrivateKey())
	if err != nil {
		// If there is an error loading the certificate and key pair, panic with the error
		panic(fmt.Errorf("failed to load X509 key pair: %v", err))
	}

	// Create a new certificate pool to hold trusted root CA certificates
	certPool := x509.NewCertPool()

	// Attempt to add the root CA certificate (in PEM format) to the certificate pool
	if !certPool.AppendCertsFromPEM(certProvider.GetRootCACertificate()) {
		panic("failed to append root CA certificate to pool")
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
