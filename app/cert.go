// Package app provides core functionality for the Lynx application framework.
package app

// CertificateProvider defines an interface for managing TLS/SSL certificates.
// It provides methods to access the certificate, private key, and root Certificate Authority (CA)
// certificate required for secure communication.
//
// CertificateProvider 定义了管理 TLS/SSL 证书的接口。
// 它提供了访问证书、私钥和根证书颁发机构（CA）证书的方法，
// 这些都是安全通信所必需的。
type CertificateProvider interface {
	// GetCertificate returns the TLS/SSL certificate as a byte slice.
	// The certificate should be in PEM format.
	// GetCertificate 返回 PEM 格式的 TLS/SSL 证书字节切片。
	GetCertificate() []byte

	// GetPrivateKey returns the private key associated with the certificate as a byte slice.
	// The private key should be in PEM format.
	// GetPrivateKey 返回 PEM 格式的与证书关联的私钥字节切片。
	GetPrivateKey() []byte

	// GetRootCACertificate returns the root CA certificate as a byte slice.
	// The root CA certificate should be in PEM format.
	// This certificate is used to verify the trust chain.
	// GetRootCACertificate 返回 PEM 格式的根 CA 证书字节切片。
	// 此证书用于验证信任链。
	GetRootCACertificate() []byte
}

// Certificate returns the current application's certificate provider.
// Returns nil if no certificate provider has been set.
//
// Certificate 返回当前应用的证书提供者。
// 如果未设置证书提供者，则返回 nil。
func (a *LynxApp) Certificate() CertificateProvider {
	return a.cert
}

// SetCertificateProvider configures the certificate provider for the application.
// The provider parameter must implement the CertificateProvider interface.
//
// SetCertificateProvider 配置应用的证书提供者。
// provider 参数必须实现 CertificateProvider 接口。
func (a *LynxApp) SetCertificateProvider(provider CertificateProvider) {
	a.cert = provider
}
