package tls

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-lynx/lynx/tls/conf"
)

// ConfigValidator validates TLS configuration
type ConfigValidator struct{}

// NewConfigValidator creates a new config validator
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{}
}

// Validate validates the TLS configuration
func (v *ConfigValidator) Validate(config *conf.Tls) error {
	if config == nil {
		return fmt.Errorf("TLS configuration is nil")
	}

	// Validate source type
	if !conf.IsValidSourceType(config.SourceType) {
		return fmt.Errorf("invalid source type: %s", config.SourceType)
	}

	// Validate based on source type
	switch config.SourceType {
	case conf.SourceTypeLocalFile:
		return v.validateLocalFileConfig(config)
	case conf.SourceTypeControlPlane:
		return v.validateControlPlaneConfig(config)
	case conf.SourceTypeMemory:
		return v.validateMemoryConfig(config)
	case conf.SourceTypeAuto:
		return nil // Auto source uses optional AutoConfig, validated at runtime
	default:
		return fmt.Errorf("unsupported source type: %s", config.SourceType)
	}
}

// validateLocalFileConfig validates local file configuration
func (v *ConfigValidator) validateLocalFileConfig(config *conf.Tls) error {
	if config.LocalFile == nil {
		return fmt.Errorf("local file configuration is nil")
	}

	// Validate required fields
	if config.LocalFile.CertFile == "" {
		return fmt.Errorf("certificate file path is required")
	}
	if config.LocalFile.KeyFile == "" {
		return fmt.Errorf("private key file path is required")
	}

	// Validate file paths
	if err := v.validateFilePath(config.LocalFile.CertFile, "certificate"); err != nil {
		return err
	}
	if err := v.validateFilePath(config.LocalFile.KeyFile, "private key"); err != nil {
		return err
	}
	if config.LocalFile.RootCaFile != "" {
		if err := v.validateFilePath(config.LocalFile.RootCaFile, "root CA"); err != nil {
			return err
		}
	}

	// Validate certificate format
	if config.LocalFile.CertFormat != "" && !conf.IsValidCertFormat(config.LocalFile.CertFormat) {
		return fmt.Errorf("invalid certificate format: %s", config.LocalFile.CertFormat)
	}

	// Validate reload interval if file watching is enabled
	if config.LocalFile.WatchFiles && config.LocalFile.ReloadInterval != nil {
		interval := config.LocalFile.ReloadInterval.AsDuration()
		if !conf.IsValidReloadInterval(interval) {
			return fmt.Errorf("reload interval must be between %v and %v",
				conf.GetMinReloadInterval().AsDuration(),
				conf.GetMaxReloadInterval().AsDuration())
		}
	}

	return nil
}

// validateControlPlaneConfig validates control plane configuration
func (v *ConfigValidator) validateControlPlaneConfig(config *conf.Tls) error {
	if config.FileName == "" {
		return fmt.Errorf("file name is required for control plane source")
	}
	return nil
}

// validateMemoryConfig validates memory configuration
func (v *ConfigValidator) validateMemoryConfig(config *conf.Tls) error {
	if config.Memory == nil {
		return fmt.Errorf("memory configuration is nil")
	}

	if config.Memory.CertData == "" {
		return fmt.Errorf("certificate data is required")
	}
	if config.Memory.KeyData == "" {
		return fmt.Errorf("private key data is required")
	}

	return nil
}

// validateFilePath validates if a file path exists and is readable
func (v *ConfigValidator) validateFilePath(filePath, fileType string) error {
	// Resolve absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path for %s %s: %w", fileType, filePath, err)
	}

	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("%s file does not exist: %s", fileType, absPath)
	}

	// Check if file is readable
	file, err := os.Open(absPath)
	if err != nil {
		return fmt.Errorf("failed to open %s file %s: %w", fileType, absPath, err)
	}
	defer file.Close()

	return nil
}

// ValidateCommonConfig validates common TLS configuration
func (v *ConfigValidator) ValidateCommonConfig(common *conf.CommonConfig) error {
	if common == nil {
		return nil // Common config is optional
	}

	// Validate authentication type
	if !conf.IsValidAuthType(common.AuthType) {
		return fmt.Errorf("invalid authentication type: %d", common.AuthType)
	}

	// Validate TLS version
	if common.MinTlsVersion != "" && !conf.IsValidTLSVersion(common.MinTlsVersion) {
		return fmt.Errorf("invalid minimum TLS version: %s", common.MinTlsVersion)
	}

	// Validate session cache size
	if common.SessionCacheSize < 0 {
		return fmt.Errorf("session cache size cannot be negative")
	}
	if common.SessionCacheSize > 10000 {
		return fmt.Errorf("session cache size cannot exceed 10,000")
	}

	return nil
}

// ValidateCompleteConfig validates the complete TLS configuration
func (v *ConfigValidator) ValidateCompleteConfig(config *conf.Tls) error {
	// Validate basic configuration
	if err := v.Validate(config); err != nil {
		return fmt.Errorf("basic validation failed: %w", err)
	}

	// Validate common configuration
	if err := v.ValidateCommonConfig(config.Common); err != nil {
		return fmt.Errorf("common configuration validation failed: %w", err)
	}

	return nil
}

// ValidateAutoConfig validates optional auto certificate configuration.
// Returns nil if autoConfig is nil.
func (v *ConfigValidator) ValidateAutoConfig(autoConfig *conf.AutoConfig) error {
	if autoConfig == nil {
		return nil
	}
	if autoConfig.RotationInterval != "" {
		d, err := time.ParseDuration(autoConfig.RotationInterval)
		if err != nil {
			return fmt.Errorf("invalid rotation_interval: %w", err)
		}
		if d < conf.MinAutoRotationInterval || d > conf.MaxAutoRotationInterval {
			return fmt.Errorf("rotation_interval must be between %v and %v", conf.MinAutoRotationInterval, conf.MaxAutoRotationInterval)
		}
	}
	if err := v.ValidateSharedCAConfig(autoConfig.SharedCA); err != nil {
		return err
	}
	return nil
}

// ValidateSharedCAConfig validates shared CA configuration when present.
// Returns nil if shared is nil.
func (v *ConfigValidator) ValidateSharedCAConfig(shared *conf.SharedCAConfig) error {
	if shared == nil {
		return nil
	}
	if shared.From == "" {
		return fmt.Errorf("shared_ca.from is required (file or control_plane)")
	}
	switch shared.From {
	case conf.SharedCAFromFile:
		if shared.CertFile == "" {
			return fmt.Errorf("shared_ca.cert_file is required when from=file")
		}
		if shared.KeyFile == "" {
			return fmt.Errorf("shared_ca.key_file is required when from=file")
		}
		if err := v.validateFilePath(shared.CertFile, "shared_ca cert"); err != nil {
			return err
		}
		if err := v.validateFilePath(shared.KeyFile, "shared_ca key"); err != nil {
			return err
		}
	case conf.SharedCAFromControlPlane:
		if shared.ConfigName == "" {
			return fmt.Errorf("shared_ca.config_name is required when from=control_plane")
		}
	default:
		return fmt.Errorf("shared_ca.from must be %q or %q", conf.SharedCAFromFile, conf.SharedCAFromControlPlane)
	}
	return nil
}
