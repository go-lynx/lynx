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
	lynxapp "github.com/go-lynx/lynx"
	"github.com/go-lynx/lynx/log"
	"github.com/go-lynx/lynx/tls/conf"
)

// CertificateManager manages TLS certificates from multiple sources
type CertificateManager struct {
	mu sync.RWMutex

	// Configuration
	config *conf.Tls

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

	// Load certificates based on source type
	var err error
	switch cm.config.SourceType {
	case conf.SourceTypeLocalFile:
		err = cm.loadFromLocalFiles()
	case conf.SourceTypeControlPlane:
		err = cm.loadFromControlPlane()
	case conf.SourceTypeMemory:
		err = cm.loadFromMemory()
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
	app := lynxapp.Lynx()
	if app == nil {
		return fmt.Errorf("lynx application not initialized")
	}
	cp := app.GetControlPlane()
	if cp == nil {
		return fmt.Errorf("control plane not available")
	}
	if cm.config.GetFileName() == "" {
		return fmt.Errorf("file name is required for control plane source")
	}
	group := cm.config.GetGroup()
	if group == "" {
		group = cm.config.GetFileName()
	}
	cfgSource, err := cp.GetConfig(cm.config.GetFileName(), group)
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

// buildTLSConfig builds the TLS configuration from loaded certificates
func (cm *CertificateManager) buildTLSConfig() error {
	// Validate certificate and private key
	if len(cm.certificate) == 0 {
		return fmt.Errorf("certificate data is empty")
	}
	if len(cm.privateKey) == 0 {
		return fmt.Errorf("private key data is empty")
	}

	// Load X.509 key pair
	tlsCert, err := tls.X509KeyPair(cm.certificate, cm.privateKey)
	if err != nil {
		return fmt.Errorf("failed to load X509 key pair: %w", err)
	}

	// Create certificate pool for root CA
	var certPool *x509.CertPool
	if len(cm.rootCA) > 0 {
		certPool = x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(cm.rootCA) {
			return fmt.Errorf("failed to append root CA certificate to pool")
		}
	}

	// Build TLS configuration
	cm.tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		ClientCAs:    certPool,
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

// reloadCertificates reloads certificates from files
func (cm *CertificateManager) reloadCertificates() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Reload certificates
	if err := cm.loadFromLocalFiles(); err != nil {
		return err
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
