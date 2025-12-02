package tls

import (
	_ "database/sql"
	"fmt"

	"github.com/go-kratos/kratos/v2/config"
	lynxapp "github.com/go-lynx/lynx"
	"github.com/go-lynx/lynx/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/tls/conf"
)

// LoaderTls represents the TLS certificate loader plugin
type LoaderTls struct {
	*plugins.BasePlugin
	tls    *conf.Tls
	cert   *conf.Cert
	weight int

	// New certificate manager
	certManager *CertificateManager
}

// GetCertificate returns the TLS/SSL certificate in PEM format
func (t *LoaderTls) GetCertificate() []byte {
	// Use new certificate manager if available
	if t.certManager != nil && t.certManager.IsInitialized() {
		return t.certManager.GetCertificate()
	}

	// Fallback to old implementation for backward compatibility
	return []byte(t.cert.GetCrt())
}

// GetPrivateKey returns the private key in PEM format
func (t *LoaderTls) GetPrivateKey() []byte {
	// Use new certificate manager if available
	if t.certManager != nil && t.certManager.IsInitialized() {
		return t.certManager.GetPrivateKey()
	}

	// Fallback to old implementation for backward compatibility
	return []byte(t.cert.GetKey())
}

// GetRootCACertificate returns the root CA certificate in PEM format
func (t *LoaderTls) GetRootCACertificate() []byte {
	// Use new certificate manager if available
	if t.certManager != nil && t.certManager.IsInitialized() {
		return t.certManager.GetRootCACertificate()
	}

	// Fallback to old implementation for backward compatibility
	return []byte(t.cert.GetRootCA())
}

// Option defines the function type for plugin options
type Option func(t *LoaderTls)

// Weight sets the plugin weight
func Weight(w int) Option {
	return func(t *LoaderTls) {
		t.weight = w
	}
}

// Config sets the TLS configuration
func Config(tls *conf.Tls) Option {
	return func(t *LoaderTls) {
		t.tls = tls
	}
}

// InitializeResources implements custom initialization for TLS loader plugin
func (t *LoaderTls) InitializeResources(rt plugins.Runtime) error {
	if t.tls == nil {
		t.tls = &conf.Tls{
			SourceType: conf.DefaultSourceType,
			FileName:   "",
			Group:      "",
		}
	}

	// Set default source type if not specified
	if t.tls.SourceType == "" {
		t.tls.SourceType = conf.DefaultSourceType
	}

	err := rt.GetConfig().Scan(t.tls)
	if err != nil {
		return err
	}
	return nil
}

// StartupTasks performs necessary tasks during plugin startup
func (t *LoaderTls) StartupTasks() error {
	// Initialize new certificate manager for all source types
	t.certManager = NewCertificateManager(t.tls)

	// Initialize certificate manager
	if err := t.certManager.Initialize(); err != nil {
		// For backward compatibility, try old method if new method fails
		if t.tls.SourceType == conf.SourceTypeControlPlane {
			log.Warnf("New certificate manager failed, falling back to old control plane method: %v", err)
			return t.startupOldMethod()
		}
		return fmt.Errorf("failed to initialize certificate manager: %w", err)
	}

	// Set certificate provider
	lynxapp.Lynx().SetCertificateProvider(t)
	log.Infof("TLS Certificate Loaded successfully using new certificate manager")
	return nil
}

// startupOldMethod implements the old control plane loading method for backward compatibility
func (t *LoaderTls) startupOldMethod() error {
	if t.tls.GetFileName() == "" {
		return nil
	}

	log.Infof("TLS Certificate Loading using old control plane method")
	cfg, err := lynxapp.Lynx().GetControlPlane().GetConfig(t.tls.GetFileName(), t.tls.GetGroup())
	if err != nil {
		return err
	}

	c := config.New(config.WithSource(cfg))
	if err := c.Load(); err != nil {
		return err
	}

	err = c.Scan(&t.cert)
	if err != nil {
		return err
	}

	lynxapp.Lynx().SetCertificateProvider(t)
	log.Infof("TLS Certificate Loaded successfully using old method")
	return nil
}

// CleanupTasks implements custom cleanup logic for TLS loader plugin
func (t *LoaderTls) CleanupTasks() error {
	// Stop certificate manager if available
	if t.certManager != nil {
		t.certManager.Stop()
	}

	lynxapp.Lynx().SetCertificateProvider(nil)
	return nil
}

// NewTlsLoader creates a new TLS loader plugin instance
func NewTlsLoader(opts ...Option) plugins.Plugin {
	t := &LoaderTls{
		weight: 100,
		tls:    &conf.Tls{},
		cert:   &conf.Cert{},
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}
