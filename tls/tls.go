package tls

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/config"
	lynxapp "github.com/go-lynx/lynx"
	"github.com/go-lynx/lynx/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/tls/conf"
)

// Plugin metadata constants
const (
	tlsPluginID          = "lynx.tls"
	tlsPluginName        = "tls"
	tlsPluginDescription = "TLS certificate loader plugin for Lynx framework"
	tlsPluginVersion     = "v2.0.0"
	tlsConfPrefix        = "lynx.tls"
	// TLSConfKeyAuto is the config key for auto-generated certificate options (e.g. rotation_interval, service_name).
	TLSConfKeyAuto = "lynx.tls.auto"
)

// LoaderTls represents the TLS certificate loader plugin
type LoaderTls struct {
	*plugins.BasePlugin
	tls    *conf.Tls
	cert   *conf.Cert
	weight int
	app    *lynxapp.LynxApp

	// autoConfig is used when source_type is "auto" for rotation and SANs.
	autoConfig *conf.AutoConfig

	certManager *CertificateManager
}

// GetCertificate returns the TLS certificate in PEM format. It prefers the
// certificate manager (which reflects rotations) and falls back to the legacy
// statically-scanned cert for backward compatibility.
func (t *LoaderTls) GetCertificate() []byte {
	if t.certManager != nil && t.certManager.IsInitialized() {
		return t.certManager.GetCertificate()
	}
	return []byte(t.cert.GetCrt())
}

// GetPrivateKey returns the private key in PEM format. See GetCertificate for the
// manager-vs-legacy fallback behaviour.
func (t *LoaderTls) GetPrivateKey() []byte {
	if t.certManager != nil && t.certManager.IsInitialized() {
		return t.certManager.GetPrivateKey()
	}
	return []byte(t.cert.GetKey())
}

// GetRootCACertificate returns the root CA certificate in PEM format. See
// GetCertificate for the manager-vs-legacy fallback behaviour.
func (t *LoaderTls) GetRootCACertificate() []byte {
	if t.certManager != nil && t.certManager.IsInitialized() {
		return t.certManager.GetRootCACertificate()
	}
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

// App binds the TLS plugin to a specific Lynx app instance.
func App(app *lynxapp.LynxApp) Option {
	return func(t *LoaderTls) {
		t.app = app
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

	if t.tls.SourceType == "" {
		t.tls.SourceType = conf.DefaultSourceType
	}

	err := rt.GetConfig().Scan(t.tls)
	if err != nil {
		return err
	}
	// Auto mode optionally reads rotation/SAN settings from TLSConfKeyAuto.
	if t.tls.SourceType == conf.SourceTypeAuto {
		var auto conf.AutoConfig
		if scanErr := rt.GetConfig().Value(TLSConfKeyAuto).Scan(&auto); scanErr == nil {
			t.autoConfig = &auto
		}
	}
	return nil
}

// StartupTasks builds and initializes the certificate manager, then registers
// this loader as the app's certificate provider. If the manager fails for a
// control-plane source it falls back to the legacy loading path.
func (t *LoaderTls) StartupTasks() error {
	if t.tls.SourceType == conf.SourceTypeAuto {
		t.certManager = NewCertificateManagerWithAuto(t.tls, t.autoConfig)
	} else {
		t.certManager = NewCertificateManager(t.tls)
	}
	t.certManager.SetControlPlaneConfigLoader(t.controlPlaneConfigLoader())
	t.certManager.SetIdentity(t.currentIdentity())

	if err := t.certManager.Initialize(); err != nil {
		if t.tls.SourceType == conf.SourceTypeControlPlane {
			log.Warnf("New certificate manager failed, falling back to old control plane method: %v", err)
			return t.startupOldMethod()
		}
		return fmt.Errorf("failed to initialize certificate manager: %w", err)
	}

	app := t.resolveApp()
	if app == nil {
		return fmt.Errorf("lynx application not initialized")
	}
	app.SetCertificateProvider(t)
	log.Infof("TLS Certificate Loaded successfully using new certificate manager")
	return nil
}

// startupOldMethod implements the old control plane loading method for backward compatibility
func (t *LoaderTls) startupOldMethod() error {
	if t.tls.GetFileName() == "" {
		return nil
	}

	log.Infof("TLS Certificate Loading using old control plane method")
	loader := t.controlPlaneConfigLoader()
	if loader == nil {
		return fmt.Errorf("control plane config loader is not available")
	}
	cfg, err := loader(t.tls.GetFileName(), t.tls.GetGroup())
	if err != nil {
		return err
	}

	c := config.New(config.WithSource(cfg))
	defer func() {
		if err := c.Close(); err != nil {
			log.Warnf("Failed to close TLS control plane config: %v", err)
		}
	}()
	if err := c.Load(); err != nil {
		return err
	}

	err = c.Scan(&t.cert)
	if err != nil {
		return err
	}

	app := t.resolveApp()
	if app == nil {
		return fmt.Errorf("lynx application not initialized")
	}
	app.SetCertificateProvider(t)
	log.Infof("TLS Certificate Loaded successfully using old method")
	return nil
}

// CleanupTasks stops certificate rotation/watching and unregisters the provider.
func (t *LoaderTls) CleanupTasks() error {
	if t.certManager != nil {
		t.certManager.Stop()
	}

	if app := t.resolveApp(); app != nil {
		app.SetCertificateProvider(nil)
	}
	return nil
}

func (t *LoaderTls) resolveApp() *lynxapp.LynxApp {
	return t.app
}

func (t *LoaderTls) controlPlaneConfigLoader() func(string, string) (config.Source, error) {
	app := t.resolveApp()
	if app == nil {
		return nil
	}
	return func(fileName, group string) (config.Source, error) {
		cp := app.GetControlPlane()
		if cp == nil {
			return nil, fmt.Errorf("control plane not available")
		}
		return cp.GetConfig(fileName, group)
	}
}

func (t *LoaderTls) currentIdentity() (string, string) {
	app := t.resolveApp()
	if app == nil {
		return "", ""
	}
	return app.Name(), app.Host()
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
	t.BasePlugin = plugins.NewBasePlugin(tlsPluginID, tlsPluginName, tlsPluginDescription, tlsPluginVersion, tlsConfPrefix, t.weight)
	return t
}
