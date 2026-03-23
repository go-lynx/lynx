package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/log"
	"github.com/go-lynx/lynx/tls/conf"
)

// CertificateManager manages TLS certificates from multiple sources
type CertificateManager struct {
	mu sync.RWMutex

	// Configuration
	config *conf.Tls

	// AutoConfig is used when source_type is "auto" for generating and rotating certificates
	autoConfig *conf.AutoConfig

	// Explicit runtime dependencies injected by the owning TLS plugin/app.
	controlPlaneConfigLoader func(string, string) (config.Source, error)
	serviceName              string
	hostname                 string

	// Current certificates
	certificate []byte
	privateKey  []byte
	rootCA      []byte

	// TLS configuration
	tlsConfig *tls.Config

	// File monitoring
	watchTicker *time.Ticker
	stopChan    chan struct{}
	watcher     *FileWatcher // Added watcher field

	// State
	initialized bool
	lastError   error
}

// NewCertificateManager creates a new certificate manager instance
func NewCertificateManager(config *conf.Tls) *CertificateManager {
	return &CertificateManager{
		config:   config,
		stopChan: make(chan struct{}),
	}
}

// NewCertificateManagerWithAuto creates a certificate manager with auto-generated certificate support.
// When config.SourceType is conf.SourceTypeAuto, autoConfig is used for rotation interval and SANs.
// autoConfig may be nil to use defaults.
func NewCertificateManagerWithAuto(config *conf.Tls, autoConfig *conf.AutoConfig) *CertificateManager {
	cm := NewCertificateManager(config)
	cm.autoConfig = autoConfig
	return cm
}

// SetControlPlaneConfigLoader injects a control plane config loader for control-plane backed certificates.
func (cm *CertificateManager) SetControlPlaneConfigLoader(loader func(string, string) (config.Source, error)) {
	cm.controlPlaneConfigLoader = loader
}

// SetIdentity injects the application identity used by auto-generated certificates.
func (cm *CertificateManager) SetIdentity(serviceName, hostname string) {
	cm.serviceName = serviceName
	cm.hostname = hostname
}

// Initialize initializes the certificate manager and loads certificates
func (cm *CertificateManager) Initialize() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.initialized {
		return nil
	}

	// Set default source type if not specified
	if cm.config.SourceType == "" {
		cm.config.SourceType = conf.DefaultSourceType
	}

	// Validate configuration
	validator := NewConfigValidator()
	if err := validator.ValidateCompleteConfig(cm.config); err != nil {
		cm.lastError = err
		return fmt.Errorf("configuration validation failed: %w", err)
	}
	if cm.config.SourceType == conf.SourceTypeAuto {
		if err := validator.ValidateAutoConfig(cm.autoConfig); err != nil {
			cm.lastError = err
			return fmt.Errorf("auto config validation failed: %w", err)
		}
	}

	// Load certificates based on source type
	var err error
	switch cm.config.SourceType {
	case conf.SourceTypeLocalFile:
		err = cm.loadFromLocalFiles()
	case conf.SourceTypeControlPlane:
		err = cm.loadFromControlPlane()
	case conf.SourceTypeMemory:
		err = cm.loadFromMemory()
	case conf.SourceTypeAuto:
		err = cm.loadFromAuto()
	default:
		return fmt.Errorf("unsupported source type: %s", cm.config.SourceType)
	}

	if err != nil {
		cm.lastError = err
		return fmt.Errorf("failed to load certificates: %w", err)
	}

	// Build TLS configuration
	if err := cm.buildTLSConfig(); err != nil {
		cm.lastError = err
		return fmt.Errorf("failed to build TLS config: %w", err)
	}

	// Start file monitoring if enabled
	if cm.config.SourceType == conf.SourceTypeLocalFile &&
		cm.config.LocalFile != nil &&
		cm.config.LocalFile.WatchFiles {
		cm.startFileMonitoring()
	}

	// Start auto certificate rotation if source is auto
	if cm.config.SourceType == conf.SourceTypeAuto {
		cm.startAutoRotation()
	}

	cm.initialized = true
	log.Infof("Certificate manager initialized successfully with source type: %s", cm.config.SourceType)
	return nil
}

// loadFromLocalFiles loads certificates from local files
func (cm *CertificateManager) loadFromLocalFiles() error {
	if cm.config.LocalFile == nil {
		return fmt.Errorf("local file configuration is nil")
	}

	// Load certificate file
	if cm.config.LocalFile.CertFile == "" {
		return fmt.Errorf("certificate file path is required")
	}
	certData, err := cm.readFile(cm.config.LocalFile.CertFile)
	if err != nil {
		return fmt.Errorf("failed to read certificate file: %w", err)
	}
	cm.certificate = certData

	// Load private key file
	if cm.config.LocalFile.KeyFile == "" {
		return fmt.Errorf("private key file path is required")
	}
	keyData, err := cm.readFile(cm.config.LocalFile.KeyFile)
	if err != nil {
		return fmt.Errorf("failed to read private key file: %w", err)
	}
	cm.privateKey = keyData

	// Load root CA file (optional)
	if cm.config.LocalFile.RootCaFile != "" {
		rootCAData, err := cm.readFile(cm.config.LocalFile.RootCaFile)
		if err != nil {
			log.Warnf("Failed to read root CA file: %v, continuing without root CA", err)
		} else {
			cm.rootCA = rootCAData
		}
	}

	log.Infof("Certificates loaded from local files: cert=%s, key=%s, rootCA=%s",
		cm.config.LocalFile.CertFile, cm.config.LocalFile.KeyFile, cm.config.LocalFile.RootCaFile)
	return nil
}

// loadFromControlPlane loads certificates from control plane via Lynx GetConfig
func (cm *CertificateManager) loadFromControlPlane() error {
	if cm.controlPlaneConfigLoader == nil {
		return fmt.Errorf("control plane config loader is not configured")
	}
	if cm.config.GetFileName() == "" {
		return fmt.Errorf("file name is required for control plane source")
	}
	group := cm.config.GetGroup()
	if group == "" {
		group = cm.config.GetFileName()
	}
	cfgSource, err := cm.controlPlaneConfigLoader(cm.config.GetFileName(), group)
	if err != nil {
		return fmt.Errorf("failed to get config from control plane: %w", err)
	}
	c := config.New(config.WithSource(cfgSource))
	if err := c.Load(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	var cert conf.Cert
	if err := c.Scan(&cert); err != nil {
		return fmt.Errorf("failed to scan cert config: %w", err)
	}
	cm.certificate = []byte(cert.GetCrt())
	cm.privateKey = []byte(cert.GetKey())
	cm.rootCA = []byte(cert.GetRootCA())
	log.Infof("Certificates loaded from control plane: file=%s group=%s", cm.config.GetFileName(), group)
	return nil
}

// loadFromMemory loads certificates from memory content
func (cm *CertificateManager) loadFromMemory() error {
	if cm.config.Memory == nil {
		return fmt.Errorf("memory configuration is nil")
	}

	if cm.config.Memory.CertData == "" {
		return fmt.Errorf("certificate data is required")
	}
	cm.certificate = []byte(cm.config.Memory.CertData)

	if cm.config.Memory.KeyData == "" {
		return fmt.Errorf("private key data is required")
	}
	cm.privateKey = []byte(cm.config.Memory.KeyData)

	if cm.config.Memory.RootCaData != "" {
		cm.rootCA = []byte(cm.config.Memory.RootCaData)
	}

	log.Infof("Certificates loaded from memory configuration")
	return nil
}

// loadFromAuto generates server certificate in-process. When SharedCA is set, uses that CA to sign the server cert
// so the mesh shares one root CA; otherwise generates a new CA and server cert per process.
func (cm *CertificateManager) loadFromAuto() error {
	cfg := cm.autoConfig
	if cfg == nil {
		cfg = &conf.AutoConfig{}
	}
	serviceName := cm.serviceName
	hostname := cm.hostname
	if serviceName == "" && cfg.ServiceName != "" {
		serviceName = cfg.ServiceName
	}
	if hostname == "" && cfg.Hostname != "" {
		hostname = cfg.Hostname
	}
	if hostname == "" {
		h, err := os.Hostname()
		if err != nil {
			hostname = "localhost"
		} else {
			hostname = h
		}
	}

	var result *CertGenResult
	var err error
	if cfg.SharedCA != nil {
		caCertPEM, caKeyPEM, loadErr := cm.loadSharedCA(cfg.SharedCA)
		if loadErr != nil {
			return fmt.Errorf("load shared CA: %w", loadErr)
		}
		validity := cfg.ParseAutoCertValidity()
		result, err = GenerateServerCertFromCA(caCertPEM, caKeyPEM, serviceName, hostname, cfg.SANs, validity)
		if err != nil {
			return fmt.Errorf("generate server cert from shared CA: %w", err)
		}
		log.Infof("Auto server certificate generated from shared CA: service=%s hostname=%s", serviceName, hostname)
	} else {
		result, err = GenerateAutoCertificatesFromConfig(cfg, serviceName, hostname)
		if err != nil {
			return fmt.Errorf("generate auto certificates: %w", err)
		}
		log.Infof("Auto certificates generated: service=%s hostname=%s", serviceName, hostname)
	}

	cm.certificate = result.CertPEM
	cm.privateKey = result.KeyPEM
	cm.rootCA = result.RootCAPEM
	return nil
}

// loadSharedCA loads the shared root CA cert and key from file or control plane.
func (cm *CertificateManager) loadSharedCA(shared *conf.SharedCAConfig) (caCertPEM, caKeyPEM []byte, err error) {
	if shared == nil || shared.From == "" {
		return nil, nil, fmt.Errorf("shared_ca.from is required")
	}
	switch shared.From {
	case conf.SharedCAFromFile:
		if shared.CertFile == "" || shared.KeyFile == "" {
			return nil, nil, fmt.Errorf("shared_ca cert_file and key_file are required when from=file")
		}
		caCertPEM, err = cm.readFile(shared.CertFile)
		if err != nil {
			return nil, nil, fmt.Errorf("read shared CA cert file: %w", err)
		}
		caKeyPEM, err = cm.readFile(shared.KeyFile)
		if err != nil {
			return nil, nil, fmt.Errorf("read shared CA key file: %w", err)
		}
		return caCertPEM, caKeyPEM, nil
	case conf.SharedCAFromControlPlane:
		if shared.ConfigName == "" {
			return nil, nil, fmt.Errorf("shared_ca config_name is required when from=control_plane")
		}
		if cm.controlPlaneConfigLoader == nil {
			return nil, nil, fmt.Errorf("control plane config loader is not configured")
		}
		group := shared.ConfigGroup
		if group == "" {
			group = shared.ConfigName
		}
		cfgSource, err := cm.controlPlaneConfigLoader(shared.ConfigName, group)
		if err != nil {
			return nil, nil, fmt.Errorf("get shared CA config from control plane: %w", err)
		}
		c := config.New(config.WithSource(cfgSource))
		if err := c.Load(); err != nil {
			return nil, nil, fmt.Errorf("load shared CA config: %w", err)
		}
		var cert conf.Cert
		if err := c.Scan(&cert); err != nil {
			return nil, nil, fmt.Errorf("scan shared CA config: %w", err)
		}
		// Control plane config: crt = CA cert, key = CA key (rootCA can be same or empty)
		caCertPEM = []byte(cert.GetCrt())
		caKeyPEM = []byte(cert.GetKey())
		if len(caCertPEM) == 0 || len(caKeyPEM) == 0 {
			return nil, nil, fmt.Errorf("shared CA config must contain crt and key")
		}
		return caCertPEM, caKeyPEM, nil
	default:
		return nil, nil, fmt.Errorf("shared_ca.from must be %q or %q", conf.SharedCAFromFile, conf.SharedCAFromControlPlane)
	}
}

// startAutoRotation starts a goroutine that rotates the server certificate at the configured interval.
func (cm *CertificateManager) startAutoRotation() {
	cfg := cm.autoConfig
	if cfg == nil {
		cfg = &conf.AutoConfig{}
	}
	interval := cfg.ParseAutoRotationInterval()
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-cm.stopChan:
				return
			case <-ticker.C:
				log.Infof("Auto certificate rotation triggered")
				if err := cm.reloadCertificates(); err != nil {
					log.Errorf("Auto certificate rotation failed: %v", err)
				} else {
					log.Infof("Auto certificate rotated successfully")
				}
			}
		}
	}()
	log.Infof("Auto certificate rotation started with interval %v", interval)
}

// readFile reads a file and returns its content
func (cm *CertificateManager) readFile(filePath string) ([]byte, error) {
	// Resolve relative paths
	if !filepath.IsAbs(filePath) {
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve absolute path for %s: %w", filePath, err)
		}
		filePath = absPath
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	// Read file content
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	return data, nil
}

// buildTLSConfig builds the TLS configuration from loaded certificates.
// Uses GetCertificate callback so that after rotation (e.g. every 24h), new TLS handshakes
// get the current certificate without restart; gRPC/HTTP servers using this config pick up new certs automatically.
func (cm *CertificateManager) buildTLSConfig() error {
	// Validate certificate and private key
	if len(cm.certificate) == 0 {
		return fmt.Errorf("certificate data is empty")
	}
	if len(cm.privateKey) == 0 {
		return fmt.Errorf("private key data is empty")
	}

	// Create certificate pool for root CA (client auth / mutual TLS)
	var certPool *x509.CertPool
	if len(cm.rootCA) > 0 {
		certPool = x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(cm.rootCA) {
			return fmt.Errorf("failed to append root CA certificate to pool")
		}
	}

	// Use GetCertificate so each new TLS handshake reads the current cert from the manager.
	// After rotation, new connections get the new certificate without process restart.
	cm.tlsConfig = &tls.Config{
		GetCertificate: func(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
			certPEM := cm.GetCertificate()
			keyPEM := cm.GetPrivateKey()
			if len(certPEM) == 0 || len(keyPEM) == 0 {
				return nil, fmt.Errorf("no certificate or key")
			}
			cert, err := tls.X509KeyPair(certPEM, keyPEM)
			if err != nil {
				return nil, fmt.Errorf("load X509 key pair: %w", err)
			}
			return &cert, nil
		},
		ClientCAs: certPool,
	}

	// Apply common configuration if available
	if cm.config.Common != nil {
		cm.applyCommonConfig()
	}

	return nil
}

// applyCommonConfig applies common TLS configuration options
func (cm *CertificateManager) applyCommonConfig() {
	common := cm.config.Common

	// Set client authentication type
	if common.AuthType != 0 {
		cm.tlsConfig.ClientAuth = tls.ClientAuthType(common.AuthType)
	}

	// Set minimum TLS version
	if common.MinTlsVersion != "" {
		cm.tlsConfig.MinVersion = cm.parseTLSVersion(common.MinTlsVersion)
	}

	// Set session cache size
	if common.SessionCacheSize > 0 {
		cm.tlsConfig.ClientSessionCache = tls.NewLRUClientSessionCache(int(common.SessionCacheSize))
	}
}

// parseTLSVersion parses TLS version string to uint16
func (cm *CertificateManager) parseTLSVersion(version string) uint16 {
	switch version {
	case conf.TLSVersion10:
		return tls.VersionTLS10
	case conf.TLSVersion11:
		return tls.VersionTLS11
	case conf.TLSVersion12:
		return tls.VersionTLS12
	case conf.TLSVersion13:
		return tls.VersionTLS13
	default:
		log.Warnf("Unknown TLS version: %s, using default", version)
		return tls.VersionTLS12
	}
}

// startFileMonitoring starts monitoring certificate files for changes
func (cm *CertificateManager) startFileMonitoring() {
	if cm.config.LocalFile == nil {
		return
	}

	// Set default reload interval if not specified
	reloadInterval := conf.DefaultReloadInterval
	if cm.config.LocalFile.ReloadInterval != nil {
		reloadInterval = cm.config.LocalFile.ReloadInterval.AsDuration()
	}

	// Create and configure file watcher
	cm.watcher = NewFileWatcher()

	// Add files to watch
	filesToWatch := []string{cm.config.LocalFile.CertFile, cm.config.LocalFile.KeyFile}
	if cm.config.LocalFile.RootCaFile != "" {
		filesToWatch = append(filesToWatch, cm.config.LocalFile.RootCaFile)
	}

	for _, file := range filesToWatch {
		if err := cm.watcher.AddFile(file); err != nil {
			log.Warnf("Failed to add file to watcher: %s, error: %v", file, err)
		}
	}

	// Start file monitoring
	cm.watcher.Start(reloadInterval)

	// Start monitoring goroutine
	go cm.monitorFiles(reloadInterval)
	log.Infof("File monitoring started with reload interval: %v", reloadInterval)
}

// monitorFiles monitors certificate files for changes
func (cm *CertificateManager) monitorFiles(reloadInterval time.Duration) {
	// Use file watcher for change detection
	for {
		select {
		case <-cm.watcher.changeChan:
			log.Infof("Certificate files changed, reloading...")
			if err := cm.reloadCertificates(); err != nil {
				log.Errorf("Failed to reload certificates: %v", err)
			} else {
				log.Infof("Certificates reloaded successfully")
			}
		case <-cm.stopChan:
			return
		}
	}
}

// checkFilesChanged checks if any monitored files have changed
func (cm *CertificateManager) checkFilesChanged() bool {
	if cm.watcher == nil {
		return false
	}
	return cm.watcher.HasChanged()
}

// reloadCertificates reloads certificates from the current source.
// Only local_file and auto support hot reload; control_plane and memory return an error if reload is attempted.
func (cm *CertificateManager) reloadCertificates() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	switch cm.config.SourceType {
	case conf.SourceTypeAuto:
		if err := cm.loadFromAuto(); err != nil {
			return err
		}
	case conf.SourceTypeLocalFile:
		if err := cm.loadFromLocalFiles(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("reload not supported for source type: %s", cm.config.SourceType)
	}

	// Rebuild TLS configuration
	if err := cm.buildTLSConfig(); err != nil {
		return err
	}

	return nil
}

// GetCertificate returns the current certificate data
func (cm *CertificateManager) GetCertificate() []byte {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.certificate
}

// GetPrivateKey returns the current private key data
func (cm *CertificateManager) GetPrivateKey() []byte {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.privateKey
}

// GetRootCACertificate returns the current root CA certificate data
func (cm *CertificateManager) GetRootCACertificate() []byte {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.rootCA
}

// GetTLSConfig returns the current TLS configuration
func (cm *CertificateManager) GetTLSConfig() *tls.Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.tlsConfig
}

// GetLastError returns the last error that occurred
func (cm *CertificateManager) GetLastError() error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.lastError
}

// IsInitialized returns whether the certificate manager is initialized
func (cm *CertificateManager) IsInitialized() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.initialized
}

// Stop stops the certificate manager and cleans up resources.
// Safe to call multiple times; subsequent calls are no-ops.
func (cm *CertificateManager) Stop() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm.initialized {
		return
	}

	// Stop file monitoring
	if cm.stopChan != nil {
		close(cm.stopChan)
		cm.stopChan = nil
	}

	if cm.watcher != nil {
		cm.watcher.Stop()
		cm.watcher.Close()
		cm.watcher = nil
	}

	cm.initialized = false
	log.Infof("Certificate manager stopped")
}
