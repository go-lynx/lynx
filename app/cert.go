// Package app provides core functionality for the Lynx application framework.
package app

// CertificateProvider defines an interface for managing TLS/SSL certificates.
// It provides methods to access the certificate, private key, and root Certificate Authority (CA)
// certificate required for secure communication.
type CertificateProvider interface {
	// GetCertificate returns the TLS/SSL certificate as a byte slice.
	// The certificate should be in PEM format.
	GetCertificate() []byte

	// GetPrivateKey returns the private key associated with the certificate as a byte slice.
	// The private key should be in PEM format.
	GetPrivateKey() []byte

	// GetRootCACertificate returns the root CA certificate as a byte slice.
	// The root CA certificate should be in PEM format.
	// This certificate is used to verify the trust chain.
	GetRootCACertificate() []byte
}

// Certificate returns the current application's certificate provider.
// Returns nil if no certificate provider has been set.
func (a *LynxApp) Certificate() CertificateProvider {
	return a.cert
}

// SetCertificateProvider configures the certificate provider for the application.
// The provider parameter must implement the CertificateProvider interface.
func (a *LynxApp) SetCertificateProvider(provider CertificateProvider) {
	a.cert = provider
}
