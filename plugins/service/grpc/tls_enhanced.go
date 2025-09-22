// Package grpc provides enhanced TLS configuration for gRPC clients
package grpc

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"google.golang.org/grpc/credentials"
	"gopkg.in/yaml.v3"
)

// TLSConfig represents enhanced TLS configuration
type TLSConfig struct {
	// Enabled indicates whether TLS is enabled
	Enabled bool `json:"enabled"`
	// InsecureSkipVerify controls whether a client verifies the server's certificate chain and host name
	InsecureSkipVerify bool `json:"insecure_skip_verify"`
	// ServerName is used to verify the hostname on the returned certificates
	ServerName string `json:"server_name"`
	// CertFile is the path to the client certificate file
	CertFile string `json:"cert_file"`
	// KeyFile is the path to the client private key file
	KeyFile string `json:"key_file"`
	// CAFile is the path to the CA certificate file
	CAFile string `json:"ca_file"`
	// ClientAuth specifies the client authentication type
	ClientAuth tls.ClientAuthType `json:"client_auth"`
	// MinVersion specifies the minimum TLS version
	MinVersion uint16 `json:"min_version"`
	// MaxVersion specifies the maximum TLS version
	MaxVersion uint16 `json:"max_version"`
	// CipherSuites specifies the cipher suites to use
	CipherSuites []uint16 `json:"cipher_suites"`
	// PreferServerCipherSuites controls whether the server selects the client's most preferred ciphersuite
	PreferServerCipherSuites bool `json:"prefer_server_cipher_suites"`
}

// DefaultTLSConfig returns a secure default TLS configuration
func DefaultTLSConfig() *TLSConfig {
	return &TLSConfig{
		Enabled:                  false,
		InsecureSkipVerify:       false,
		ClientAuth:               tls.NoClientCert,
		MinVersion:               tls.VersionTLS12,
		MaxVersion:               tls.VersionTLS13,
		PreferServerCipherSuites: true,
		// Only use secure cipher suites with forward secrecy
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
	}
}

// TLSManager manages TLS configurations and credentials with thread safety
type TLSManager struct {
	mu      sync.RWMutex
	configs map[string]*TLSConfig
	creds   map[string]credentials.TransportCredentials
}

// NewTLSManager creates a new TLS manager
func NewTLSManager() *TLSManager {
	return &TLSManager{
		configs: make(map[string]*TLSConfig),
		creds:   make(map[string]credentials.TransportCredentials),
	}
}

// SetServiceConfig sets TLS configuration for a specific service
func (tm *TLSManager) SetServiceConfig(serviceName string, config *TLSConfig) error {
	if strings.TrimSpace(serviceName) == "" {
		return errors.New("service name cannot be empty")
	}

	if config == nil {
		config = DefaultTLSConfig()
	}

	// Validate configuration
	if err := tm.validateConfig(config); err != nil {
		return fmt.Errorf("invalid TLS config for service %s: %w", serviceName, err)
	}

	// Build and cache credentials
	creds, err := tm.buildCredentials(config)
	if err != nil {
		return fmt.Errorf("failed to build TLS credentials for service %s: %w", serviceName, err)
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.configs[serviceName] = config
	tm.creds[serviceName] = creds
	return nil
}

// GetCredentials returns TLS credentials for a service
func (tm *TLSManager) GetCredentials(serviceName string) (credentials.TransportCredentials, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if creds, exists := tm.creds[serviceName]; exists {
		return creds, nil
	}

	// Return error instead of insecure credentials for security
	return nil, fmt.Errorf("no TLS configuration found for service %s", serviceName)
}

// GetConfig returns TLS configuration for a service
func (tm *TLSManager) GetConfig(serviceName string) *TLSConfig {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if config, exists := tm.configs[serviceName]; exists {
		return config
	}
	return DefaultTLSConfig()
}

// RemoveService removes TLS configuration for a service
func (tm *TLSManager) RemoveService(serviceName string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.configs, serviceName)
	delete(tm.creds, serviceName)
}

// validateConfig validates TLS configuration with enhanced security checks
func (tm *TLSManager) validateConfig(config *TLSConfig) error {
	if !config.Enabled {
		return nil
	}

	// Validate TLS version security
	if config.MinVersion < tls.VersionTLS12 {
		return fmt.Errorf("minimum TLS version must be at least TLS 1.2 for security")
	}

	// Validate TLS version range
	if config.MinVersion > config.MaxVersion {
		return fmt.Errorf("min TLS version (%d) cannot be greater than max version (%d)",
			config.MinVersion, config.MaxVersion)
	}

	// Validate certificate files if specified
	if config.CertFile != "" {
		if config.KeyFile == "" {
			return fmt.Errorf("key file must be specified when cert file is provided")
		}

		if !tm.fileExists(config.CertFile) {
			return fmt.Errorf("certificate file does not exist: %s", config.CertFile)
		}

		if !tm.fileExists(config.KeyFile) {
			return fmt.Errorf("key file does not exist: %s", config.KeyFile)
		}

		// Validate certificate can be loaded
		if _, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile); err != nil {
			return fmt.Errorf("invalid certificate/key pair: %w", err)
		}
	}

	// Validate CA file if specified
	if config.CAFile != "" {
		if !tm.fileExists(config.CAFile) {
			return fmt.Errorf("CA file does not exist: %s", config.CAFile)
		}

		// Validate CA certificate can be loaded
		caCert, err := os.ReadFile(config.CAFile)
		if err != nil {
			return fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return fmt.Errorf("failed to parse CA certificate")
		}
	}

	// Validate cipher suites for security
	if err := tm.validateCipherSuites(config.CipherSuites); err != nil {
		return fmt.Errorf("cipher suite validation failed: %w", err)
	}

	return nil
}

// validateCipherSuites validates that cipher suites are secure
func (tm *TLSManager) validateCipherSuites(cipherSuites []uint16) error {
	if len(cipherSuites) == 0 {
		return nil // Use default secure cipher suites
	}

	// Insecure cipher suites to reject
	insecureCipherSuites := map[uint16]string{
		tls.TLS_RSA_WITH_AES_256_GCM_SHA384: "RSA key exchange without forward secrecy",
		tls.TLS_RSA_WITH_AES_128_GCM_SHA256: "RSA key exchange without forward secrecy",
		tls.TLS_RSA_WITH_RC4_128_SHA:        "RC4 cipher is cryptographically broken",
		tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA:   "3DES cipher is weak",
	}

	var insecureFound []string
	for _, suite := range cipherSuites {
		if reason, isInsecure := insecureCipherSuites[suite]; isInsecure {
			insecureFound = append(insecureFound, fmt.Sprintf("0x%04X (%s)", suite, reason))
		}
	}

	if len(insecureFound) > 0 {
		return fmt.Errorf("insecure cipher suites detected: %s", strings.Join(insecureFound, ", "))
	}

	return nil
}

// buildCredentials builds gRPC transport credentials from TLS configuration
func (tm *TLSManager) buildCredentials(config *TLSConfig) (credentials.TransportCredentials, error) {
	if !config.Enabled {
		return nil, errors.New("TLS is disabled, cannot create secure credentials")
	}

	tlsConfig := &tls.Config{
		ServerName:         config.ServerName,
		InsecureSkipVerify: config.InsecureSkipVerify,
		ClientAuth:         config.ClientAuth,
		MinVersion:         config.MinVersion,
		MaxVersion:         config.MaxVersion,
		CipherSuites:       config.CipherSuites,
	}

	// Load client certificate if specified
	if config.CertFile != "" && config.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// Load CA certificate if specified
	if config.CAFile != "" {
		caCert, err := os.ReadFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	return credentials.NewTLS(tlsConfig), nil
}

// fileExists checks if a file exists efficiently
func (tm *TLSManager) fileExists(filename string) bool {
	if filename == "" {
		return false
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return false
	}

	// Use Stat instead of ReadFile for better performance
	_, err = os.Stat(absPath)
	return err == nil
}

// GetStats returns statistics about TLS configurations
func (tm *TLSManager) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"total_services": len(tm.configs),
		"services":       make(map[string]interface{}),
	}

	services := make(map[string]interface{})
	for serviceName, config := range tm.configs {
		services[serviceName] = map[string]interface{}{
			"enabled":                     config.Enabled,
			"insecure_skip_verify":        config.InsecureSkipVerify,
			"server_name":                 config.ServerName,
			"has_client_cert":             config.CertFile != "",
			"has_ca_cert":                 config.CAFile != "",
			"client_auth":                 config.ClientAuth.String(),
			"min_version":                 tm.tlsVersionString(config.MinVersion),
			"max_version":                 tm.tlsVersionString(config.MaxVersion),
			"cipher_suites_count":         len(config.CipherSuites),
			"prefer_server_cipher_suites": config.PreferServerCipherSuites,
		}
	}
	stats["services"] = services

	return stats
}

// tlsVersionString converts TLS version number to string
func (tm *TLSManager) tlsVersionString(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("Unknown (%d)", version)
	}
}

// Close cleans up TLS manager resources
func (tm *TLSManager) Close() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.configs = make(map[string]*TLSConfig)
	tm.creds = make(map[string]credentials.TransportCredentials)
}

// LoadConfigFromFile loads TLS configuration from a file
func (tm *TLSManager) LoadConfigFromFile(serviceName, configFile string) error {
	// Read the configuration file
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read TLS config file %s: %w", configFile, err)
	}

	var config TLSConfig

	// Determine file format by extension
	ext := filepath.Ext(configFile)
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse JSON TLS config: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse YAML TLS config: %w", err)
		}
	default:
		// Try YAML first, then JSON as fallback
		if yamlErr := yaml.Unmarshal(data, &config); yamlErr != nil {
			if jsonErr := json.Unmarshal(data, &config); jsonErr != nil {
				return fmt.Errorf("failed to parse config file (tried YAML and JSON): %w", yamlErr)
			}
		}
	}

	// Validate and resolve relative paths
	if err := tm.validateAndResolvePaths(&config, filepath.Dir(configFile)); err != nil {
		return fmt.Errorf("TLS config validation failed: %w", err)
	}

	// Store the configuration
	return tm.SetServiceConfig(serviceName, &config)
}

// validateAndResolvePaths validates TLS configuration and resolves relative paths
func (tm *TLSManager) validateAndResolvePaths(config *TLSConfig, baseDir string) error {
	if !config.Enabled {
		return nil
	}

	// Resolve relative paths to absolute paths
	if config.CertFile != "" {
		if !filepath.IsAbs(config.CertFile) {
			config.CertFile = filepath.Join(baseDir, config.CertFile)
		}
		if !tm.fileExists(config.CertFile) {
			return fmt.Errorf("certificate file not found: %s", config.CertFile)
		}
	}

	if config.KeyFile != "" {
		if !filepath.IsAbs(config.KeyFile) {
			config.KeyFile = filepath.Join(baseDir, config.KeyFile)
		}
		if !tm.fileExists(config.KeyFile) {
			return fmt.Errorf("private key file not found: %s", config.KeyFile)
		}
	}

	if config.CAFile != "" {
		if !filepath.IsAbs(config.CAFile) {
			config.CAFile = filepath.Join(baseDir, config.CAFile)
		}
		if !tm.fileExists(config.CAFile) {
			return fmt.Errorf("CA certificate file not found: %s", config.CAFile)
		}
	}

	// Validate that cert and key files are both provided or both empty
	if (config.CertFile != "" && config.KeyFile == "") || (config.CertFile == "" && config.KeyFile != "") {
		return fmt.Errorf("both certificate and private key files must be provided together")
	}

	return nil
}

// RefreshCredentials refreshes TLS credentials for all services
func (tm *TLSManager) RefreshCredentials() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	var tlsErrors []string
	successCount := 0

	for serviceName, config := range tm.configs {
		credList, err := tm.buildCredentials(config)
		if err != nil {
			tlsErrors = append(tlsErrors, fmt.Sprintf("service %s: %v", serviceName, err))
			continue
		}
		tm.creds[serviceName] = credList
		successCount++
	}

	if len(tlsErrors) > 0 {
		return fmt.Errorf("failed to refresh %d/%d services: %s", len(tlsErrors), len(tm.configs), strings.Join(tlsErrors, "; "))
	}

	return nil
}
