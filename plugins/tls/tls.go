package cert

import (
	_ "database/sql"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"

	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/plugin-tls/conf"
)

var (
	name       = "tls-loader"
	confPrefix = "lynx.application.tls"
)

// TlsLoader represents the TLS certificate loader plugin
type TlsLoader struct {
	*plugins.BasePlugin
	tls    *conf.Tls
	cert   *conf.Cert
	weight int
}

// GetCrt returns the certificate content
func (t *TlsLoader) GetCrt() []byte {
	return []byte(t.cert.GetCrt())
}

// GetKey returns the private key content
func (t *TlsLoader) GetKey() []byte {
	return []byte(t.cert.GetKey())
}

// GetRootCA returns the root CA content
func (t *TlsLoader) GetRootCA() []byte {
	return []byte(t.cert.GetRootCA())
}

// Option defines the function type for plugin options
type Option func(t *TlsLoader)

// Weight sets the plugin weight
func Weight(w int) Option {
	return func(t *TlsLoader) {
		t.weight = w
	}
}

// Config sets the TLS configuration
func Config(tls *conf.Tls) Option {
	return func(t *TlsLoader) {
		t.tls = tls
	}
}

// InitializeResources implements custom initialization for TLS loader plugin
func (t *TlsLoader) InitializeResources(rt plugins.Runtime) error {
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
func (t *TlsLoader) StartupTasks() error {
	if t.tls.GetFileName() == "" {
		return nil
	}
	app.Lynx().GetLogHelper().Infof("TLS Certificate Loading")
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

	app.Lynx().SetCert(t)
	app.Lynx().GetLogHelper().Infof("TLS Certificate Loaded successfully")
	return nil
}

// CleanupTasks implements custom cleanup logic for TLS loader plugin
func (t *TlsLoader) CleanupTasks() error {
	app.Lynx().SetCert(nil)
	return nil
}

// NewTlsLoader creates a new TLS loader plugin instance
func NewTlsLoader(opts ...Option) plugins.Plugin {
	t := &TlsLoader{
		weight: 100,
		tls:    &conf.Tls{},
		cert:   &conf.Cert{},
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}
