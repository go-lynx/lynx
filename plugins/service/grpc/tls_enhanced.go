// Package grpc provides enhanced TLS configuration for gRPC clients
package grpc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"google.golang.org/grpc/credentials"
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

// DefaultTLSConfig returns a default TLS configuration
func DefaultTLSConfig() *TLSConfig {
	return &TLSConfig{
		Enabled:            false,
		InsecureSkipVerify: false,
		ClientAuth:         tls.NoClientCert,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		},
	}
}

// TLSManager manages TLS configurations and credentials
type TLSManager struct {
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
	if config == nil {
		config = DefaultTLSConfig()
	}

	// Validate configuration
	if err := tm.validateConfig(config); err != nil {
		return fmt.Errorf("invalid TLS config for service %s: %w", serviceName, err)
	}

	tm.configs[serviceName] = config

	// Build and cache credentials
	creds, err := tm.buildCredentials(config)
	if err != nil {
		return fmt.Errorf("failed to build TLS credentials for service %s: %w", serviceName, err)
	}

	tm.creds[serviceName] = creds
	return nil
}

// GetCredentials returns TLS credentials for a service
func (tm *TLSManager) GetCredentials(serviceName string) (credentials.TransportCredentials, error) {
	if creds, exists := tm.creds[serviceName]; exists {
		return creds, nil
	}

	// Use default insecure credentials if no TLS config
	return credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}), nil
}

// GetConfig returns TLS configuration for a service
func (tm *TLSManager) GetConfig(serviceName string) *TLSConfig {
	if config, exists := tm.configs[serviceName]; exists {
		return config
	}
	return DefaultTLSConfig()
}

// RemoveService removes TLS configuration for a service
func (tm *TLSManager) RemoveService(serviceName string) {
	delete(tm.configs, serviceName)
	delete(tm.creds, serviceName)
}

// validateConfig validates TLS configuration
func (tm *TLSManager) validateConfig(config *TLSConfig) error {
	if !config.Enabled {
		return nil
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
	}

	// Validate CA file if specified
	if config.CAFile != "" && !tm.fileExists(config.CAFile) {
		return fmt.Errorf("CA file does not exist: %s", config.CAFile)
	}

	// Validate TLS version range
	if config.MinVersion > config.MaxVersion {
		return fmt.Errorf("min TLS version (%d) cannot be greater than max version (%d)", 
			config.MinVersion, config.MaxVersion)
	}

	return nil
}

// buildCredentials builds gRPC transport credentials from TLS configuration
func (tm *TLSManager) buildCredentials(config *TLSConfig) (credentials.TransportCredentials, error) {
	if !config.Enabled {
		return credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}), nil
	}

	tlsConfig := &tls.Config{
		ServerName:               config.ServerName,
		InsecureSkipVerify:      config.InsecureSkipVerify,
		ClientAuth:              config.ClientAuth,
		MinVersion:              config.MinVersion,
		MaxVersion:              config.MaxVersion,
		CipherSuites:            config.CipherSuites,
		PreferServerCipherSuites: config.PreferServerCipherSuites,
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
		caCert, err := ioutil.ReadFile(config.CAFile)
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

// fileExists checks if a file exists
func (tm *TLSManager) fileExists(filename string) bool {
	if filename == "" {
		return false
	}
	
	// Convert to absolute path
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return false
	}
	
	// Check if file exists and is readable
	_, err = ioutil.ReadFile(absPath)
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
			"enabled":                    config.Enabled,
			"insecure_skip_verify":      config.InsecureSkipVerify,
			"server_name":               config.ServerName,
			"has_client_cert":           config.CertFile != "",
			"has_ca_cert":               config.CAFile != "",
			"client_auth":               config.ClientAuth.String(),
			"min_version":               tm.tlsVersionString(config.MinVersion),
			"max_version":               tm.tlsVersionString(config.MaxVersion),
			"cipher_suites_count":       len(config.CipherSuites),
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
	tm.configs = make(map[string]*TLSConfig)
	tm.creds = make(map[string]credentials.TransportCredentials)
}

// LoadConfigFromFile loads TLS configuration from a file
func (tm *TLSManager) LoadConfigFromFile(serviceName, configFile string) error {
	// This is a placeholder for loading TLS config from external files
	// In a real implementation, you might load from JSON, YAML, or other formats
	config := DefaultTLSConfig()
	config.Enabled = true
	
	// Try to detect certificate files in the same directory
	configDir := filepath.Dir(configFile)
	certFile := filepath.Join(configDir, serviceName+".crt")
	keyFile := filepath.Join(configDir, serviceName+".key")
	caFile := filepath.Join(configDir, "ca.crt")
	
	if tm.fileExists(certFile) && tm.fileExists(keyFile) {
		config.CertFile = certFile
		config.KeyFile = keyFile
	}
	
	if tm.fileExists(caFile) {
		config.CAFile = caFile
	}
	
	return tm.SetServiceConfig(serviceName, config)
}

// RefreshCredentials refreshes TLS credentials for all services
func (tm *TLSManager) RefreshCredentials() error {
	var lastErr error
	
	for serviceName, config := range tm.configs {
		creds, err := tm.buildCredentials(config)
		if err != nil {
			lastErr = fmt.Errorf("failed to refresh credentials for service %s: %w", serviceName, err)
			continue
		}
		tm.creds[serviceName] = creds
	}
	
	return lastErr
}
