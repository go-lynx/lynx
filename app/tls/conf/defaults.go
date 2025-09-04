package conf

import (
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
)

// Default configuration constants
const (
	// Source types
	SourceTypeControlPlane = "control_plane"
	SourceTypeLocalFile    = "local_file"
	SourceTypeMemory       = "memory"

	// Certificate formats
	CertFormatPEM = "pem"
	CertFormatDER = "der"

	// TLS version constants
	TLSVersion10 = "1.0"
	TLSVersion11 = "1.1"
	TLSVersion12 = "1.2"
	TLSVersion13 = "1.3"

	// Default values (these are actual defaults, not field values)
	DefaultSourceType       = SourceTypeControlPlane
	DefaultCertFormat       = CertFormatPEM
	DefaultMinTLSVersion    = "1.2"
	DefaultAuthType         = 0
	DefaultVerifyHostname   = true
	DefaultSessionCacheSize = 32
	DefaultWatchFiles       = false
	DefaultReloadInterval   = 5 * time.Second

	// Validation limits
	MinReloadInterval   = 1 * time.Second
	MaxReloadInterval   = 300 * time.Second
	MinSessionCacheSize = 0
	MaxSessionCacheSize = 10000
)

// Supported source types
var SupportedSourceTypes = []string{
	SourceTypeControlPlane,
	SourceTypeLocalFile,
	SourceTypeMemory,
}

// Supported certificate formats
var SupportedCertFormats = []string{
	CertFormatPEM,
	CertFormatDER,
}

// Supported TLS versions
var SupportedTLSVersions = []string{
	TLSVersion10, TLSVersion11, TLSVersion12, TLSVersion13,
}

// Supported authentication types
var SupportedAuthTypes = []int32{
	0, 1, 2, 3, 4,
}

// GetDefaultReloadInterval returns the default reload interval duration
func GetDefaultReloadInterval() *durationpb.Duration {
	return &durationpb.Duration{Seconds: int64(DefaultReloadInterval.Seconds())}
}

// GetMinReloadInterval returns the minimum reload interval duration
func GetMinReloadInterval() *durationpb.Duration {
	return &durationpb.Duration{Seconds: int64(MinReloadInterval.Seconds())}
}

// GetMaxReloadInterval returns the maximum reload interval duration
func GetMaxReloadInterval() *durationpb.Duration {
	return &durationpb.Duration{Seconds: int64(MaxReloadInterval.Seconds())}
}

// IsValidSourceType checks if the source type is valid
func IsValidSourceType(sourceType string) bool {
	for _, validType := range SupportedSourceTypes {
		if validType == sourceType {
			return true
		}
	}
	return false
}

// IsValidCertFormat checks if the certificate format is valid
func IsValidCertFormat(format string) bool {
	for _, validFormat := range SupportedCertFormats {
		if validFormat == format {
			return true
		}
	}
	return false
}

// IsValidTLSVersion checks if the TLS version is valid
func IsValidTLSVersion(version string) bool {
	for _, validVersion := range SupportedTLSVersions {
		if validVersion == version {
			return true
		}
	}
	return false
}

// IsValidAuthType checks if the authentication type is valid
func IsValidAuthType(authType int32) bool {
	return authType >= 0 && authType <= 4
}

// IsValidReloadInterval checks if the reload interval is within valid range
func IsValidReloadInterval(interval time.Duration) bool {
	return interval >= MinReloadInterval && interval <= MaxReloadInterval
}

// IsValidSessionCacheSize checks if the session cache size is within valid range
func IsValidSessionCacheSize(size int32) bool {
	return size >= MinSessionCacheSize && size <= MaxSessionCacheSize
}
