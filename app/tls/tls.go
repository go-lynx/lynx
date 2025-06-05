package tls

import (
	_ "database/sql"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/app/tls/conf"
	"github.com/go-lynx/lynx/plugins"
)

// LoaderTls represents the TLS certificate loader plugin
type LoaderTls struct {
	*plugins.BasePlugin
	tls    *conf.Tls
	cert   *conf.Cert
	weight int
}

// GetCertificate returns the TLS/SSL certificate in PEM format
func (t *LoaderTls) GetCertificate() []byte {
	return []byte(t.cert.GetCrt())
}

// GetPrivateKey returns the private key in PEM format
func (t *LoaderTls) GetPrivateKey() []byte {
	return []byte(t.cert.GetKey())
}

// GetRootCACertificate returns the root CA certificate in PEM format
func (t *LoaderTls) GetRootCACertificate() []byte {
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
			FileName: "",
			Group:    "",
		}
	}
	err := rt.GetConfig().Scan(t.tls)
	if err != nil {
		return err
	}
	return nil
}

// StartupTasks performs necessary tasks during plugin startup
func (t *LoaderTls) StartupTasks() error {
	if t.tls.GetFileName() == "" {
		return nil
	}
	log.Infof("TLS Certificate Loading")
	cfg, err := app.Lynx().GetControlPlane().GetConfig(t.tls.GetFileName(), t.tls.GetGroup())
	if err != nil {
		return err
	}
	c := config.New(config.WithSource(cfg))
	if err := c.Load(); err != nil {
		return err
	}

	err = c.Scan(t.cert)
	if err != nil {
		return err
	}

	app.Lynx().SetCertificateProvider(t)
	log.Infof("TLS Certificate Loaded successfully")
	return nil
}

// CleanupTasks implements custom cleanup logic for TLS loader plugin
func (t *LoaderTls) CleanupTasks() error {
	app.Lynx().SetCertificateProvider(nil)
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
