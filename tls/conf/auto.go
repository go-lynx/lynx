package conf

import (
	"time"
)

// SourceTypeAuto is the source type for auto-generated certificates (in-process CA + server cert).
const SourceTypeAuto = "auto"

// SharedCAFrom is the source type for loading the shared root CA.
const (
	SharedCAFromFile         = "file"
	SharedCAFromControlPlane = "control_plane"
)

// SharedCAConfig configures loading a shared root CA so that all services use the same CA.
// When set, the server certificate is signed by this CA instead of generating a new CA per process.
// This allows A to subscribe to B/C/D with defaultRootCA (same CA) and verify their server certs.
type SharedCAConfig struct {
	// From is "file" or "control_plane".
	From string `json:"from,omitempty"`
	// For file: paths to the CA certificate and private key (PEM).
	CertFile string `json:"cert_file,omitempty"`
	KeyFile  string `json:"key_file,omitempty"`
	// For control_plane: config name and group to load (content: conf.Cert with crt=CA cert, key=CA key).
	ConfigName  string `json:"config_name,omitempty"`
	ConfigGroup string `json:"config_group,omitempty"`
}

// AutoConfig holds configuration for auto-generated TLS certificates.
// Used when source_type is "auto": an in-process CA and server certificate are generated on first load;
// periodic rotation reissues only the server leaf while keeping the same root (stable GetRootCACertificate).
// When shared_ca is set, that external CA is used instead and reloaded per rotation policy.
// Config can be loaded from config key "lynx.tls.auto".
type AutoConfig struct {
	// RotationInterval is the interval after which the server certificate is rotated.
	// Parsed as duration string (e.g. "24h", "1h"). Default: 24h.
	RotationInterval string `json:"rotation_interval,omitempty"`
	// ServiceName is used as CN and in SAN for the server certificate (e.g. gRPC service name).
	ServiceName string `json:"service_name,omitempty"`
	// Hostname is added to SAN (e.g. for TLS ServerName verification). If empty, os.Hostname() is used.
	Hostname string `json:"hostname,omitempty"`
	// SANs are additional Subject Alternative Names (e.g. "localhost", "127.0.0.1").
	SANs []string `json:"sans,omitempty"`
	// CertValidity is the validity duration for the server cert. Parsed as duration (e.g. "24h").
	// Should be >= RotationInterval. Default: same as RotationInterval.
	CertValidity string `json:"cert_validity,omitempty"`
	// SharedCA, when set, uses an existing root CA to sign the server cert so the mesh shares one CA.
	// GetRootCACertificate() then returns this CA; clients (e.g. A subscribing B/C/D) use it to verify all servers.
	SharedCA *SharedCAConfig `json:"shared_ca,omitempty"`
}

// DefaultAutoRotationInterval is the default rotation interval for auto-generated certs.
const DefaultAutoRotationInterval = 24 * time.Hour

// MinAutoRotationInterval is the minimum allowed rotation interval.
const MinAutoRotationInterval = 1 * time.Hour

// MaxAutoRotationInterval is the maximum allowed rotation interval.
const MaxAutoRotationInterval = 168 * time.Hour // 7 days

// ParseAutoRotationInterval parses RotationInterval string and returns duration, or default if empty/invalid.
func (a *AutoConfig) ParseAutoRotationInterval() time.Duration {
	if a == nil || a.RotationInterval == "" {
		return DefaultAutoRotationInterval
	}
	d, err := time.ParseDuration(a.RotationInterval)
	if err != nil || d < MinAutoRotationInterval || d > MaxAutoRotationInterval {
		return DefaultAutoRotationInterval
	}
	return d
}

// ParseAutoCertValidity parses CertValidity string. If empty, returns the same as rotation interval.
func (a *AutoConfig) ParseAutoCertValidity() time.Duration {
	if a == nil || a.CertValidity == "" {
		return a.ParseAutoRotationInterval()
	}
	d, err := time.ParseDuration(a.CertValidity)
	if err != nil || d < MinAutoRotationInterval {
		return a.ParseAutoRotationInterval()
	}
	return d
}
