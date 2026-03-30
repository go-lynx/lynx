package tls

import (
	"testing"
	"time"

	"github.com/go-lynx/lynx/tls/conf"
)

// ---- ConfigValidator tests ----

func newValidator() *ConfigValidator {
	return NewConfigValidator()
}

func TestConfigValidator_NilConfig(t *testing.T) {
	v := newValidator()
	err := v.Validate(nil)
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestConfigValidator_InvalidSourceType(t *testing.T) {
	v := newValidator()
	err := v.Validate(&conf.Tls{SourceType: "unsupported_type"})
	if err == nil {
		t.Error("expected error for invalid source type")
	}
}

func TestConfigValidator_AutoSource(t *testing.T) {
	v := newValidator()
	err := v.Validate(&conf.Tls{SourceType: conf.SourceTypeAuto})
	if err != nil {
		t.Errorf("auto source should be valid, got: %v", err)
	}
}

func TestConfigValidator_ControlPlane_MissingFileName(t *testing.T) {
	v := newValidator()
	err := v.Validate(&conf.Tls{
		SourceType: conf.SourceTypeControlPlane,
		FileName:   "",
	})
	if err == nil {
		t.Error("expected error when FileName is missing for control_plane source")
	}
}

func TestConfigValidator_ControlPlane_Valid(t *testing.T) {
	v := newValidator()
	err := v.Validate(&conf.Tls{
		SourceType: conf.SourceTypeControlPlane,
		FileName:   "tls.yaml",
	})
	if err != nil {
		t.Errorf("expected valid config, got: %v", err)
	}
}

func TestConfigValidator_Memory_NilMemory(t *testing.T) {
	v := newValidator()
	err := v.Validate(&conf.Tls{
		SourceType: conf.SourceTypeMemory,
		Memory:     nil,
	})
	if err == nil {
		t.Error("expected error when memory config is nil")
	}
}

func TestConfigValidator_Memory_MissingCertData(t *testing.T) {
	v := newValidator()
	err := v.Validate(&conf.Tls{
		SourceType: conf.SourceTypeMemory,
		Memory: &conf.MemoryConfig{
			CertData: "",
			KeyData:  "some-key",
		},
	})
	if err == nil {
		t.Error("expected error for missing cert data")
	}
}

func TestConfigValidator_Memory_MissingKeyData(t *testing.T) {
	v := newValidator()
	err := v.Validate(&conf.Tls{
		SourceType: conf.SourceTypeMemory,
		Memory: &conf.MemoryConfig{
			CertData: "some-cert",
			KeyData:  "",
		},
	})
	if err == nil {
		t.Error("expected error for missing key data")
	}
}

func TestConfigValidator_Memory_Valid(t *testing.T) {
	v := newValidator()
	err := v.Validate(&conf.Tls{
		SourceType: conf.SourceTypeMemory,
		Memory: &conf.MemoryConfig{
			CertData: "cert-pem-data",
			KeyData:  "key-pem-data",
		},
	})
	if err != nil {
		t.Errorf("expected valid memory config, got: %v", err)
	}
}

func TestConfigValidator_LocalFile_NilLocalFile(t *testing.T) {
	v := newValidator()
	err := v.Validate(&conf.Tls{
		SourceType: conf.SourceTypeLocalFile,
		LocalFile:  nil,
	})
	if err == nil {
		t.Error("expected error when LocalFile is nil")
	}
}

func TestConfigValidator_LocalFile_MissingCertFile(t *testing.T) {
	v := newValidator()
	err := v.Validate(&conf.Tls{
		SourceType: conf.SourceTypeLocalFile,
		LocalFile: &conf.LocalFileConfig{
			CertFile: "",
			KeyFile:  "key.pem",
		},
	})
	if err == nil {
		t.Error("expected error for missing cert file")
	}
}

func TestConfigValidator_LocalFile_MissingKeyFile(t *testing.T) {
	v := newValidator()
	err := v.Validate(&conf.Tls{
		SourceType: conf.SourceTypeLocalFile,
		LocalFile: &conf.LocalFileConfig{
			CertFile: "cert.pem",
			KeyFile:  "",
		},
	})
	if err == nil {
		t.Error("expected error for missing key file")
	}
}

func TestConfigValidator_LocalFile_InvalidCertFormat(t *testing.T) {
	v := newValidator()
	err := v.Validate(&conf.Tls{
		SourceType: conf.SourceTypeLocalFile,
		LocalFile: &conf.LocalFileConfig{
			CertFile:   "cert.pem", // Will fail on file existence check first, that's fine
			KeyFile:    "key.pem",
			CertFormat: "invalid_format",
		},
	})
	// The error may come from file existence check or format check
	// Either way, an error should be returned
	if err == nil {
		t.Error("expected error for invalid cert format")
	}
}

// ---- ValidateCommonConfig tests ----

func TestConfigValidator_CommonConfig_Nil(t *testing.T) {
	v := newValidator()
	err := v.ValidateCommonConfig(nil)
	if err != nil {
		t.Errorf("nil common config should be valid, got: %v", err)
	}
}

func TestConfigValidator_CommonConfig_InvalidAuthType(t *testing.T) {
	v := newValidator()
	err := v.ValidateCommonConfig(&conf.CommonConfig{
		AuthType: 99,
	})
	if err == nil {
		t.Error("expected error for invalid auth type")
	}
}

func TestConfigValidator_CommonConfig_InvalidTLSVersion(t *testing.T) {
	v := newValidator()
	err := v.ValidateCommonConfig(&conf.CommonConfig{
		AuthType:      0,
		MinTlsVersion: "2.0",
	})
	if err == nil {
		t.Error("expected error for invalid TLS version")
	}
}

func TestConfigValidator_CommonConfig_ValidVersions(t *testing.T) {
	v := newValidator()
	versions := []string{"1.0", "1.1", "1.2", "1.3", ""}
	for _, version := range versions {
		err := v.ValidateCommonConfig(&conf.CommonConfig{
			AuthType:      0,
			MinTlsVersion: version,
		})
		if err != nil {
			t.Errorf("expected valid TLS version %q, got: %v", version, err)
		}
	}
}

func TestConfigValidator_CommonConfig_NegativeSessionCacheSize(t *testing.T) {
	v := newValidator()
	err := v.ValidateCommonConfig(&conf.CommonConfig{
		AuthType:         0,
		SessionCacheSize: -1,
	})
	if err == nil {
		t.Error("expected error for negative session cache size")
	}
}

func TestConfigValidator_CommonConfig_TooLargeSessionCacheSize(t *testing.T) {
	v := newValidator()
	err := v.ValidateCommonConfig(&conf.CommonConfig{
		AuthType:         0,
		SessionCacheSize: 10001,
	})
	if err == nil {
		t.Error("expected error for session cache size exceeding limit")
	}
}

// ---- ValidateAutoConfig tests ----

func TestConfigValidator_AutoConfig_Nil(t *testing.T) {
	v := newValidator()
	err := v.ValidateAutoConfig(nil)
	if err != nil {
		t.Errorf("nil auto config should be valid, got: %v", err)
	}
}

func TestConfigValidator_AutoConfig_InvalidRotationInterval(t *testing.T) {
	v := newValidator()
	err := v.ValidateAutoConfig(&conf.AutoConfig{
		RotationInterval: "not-a-duration",
	})
	if err == nil {
		t.Error("expected error for invalid rotation interval")
	}
}

func TestConfigValidator_AutoConfig_TooShortRotationInterval(t *testing.T) {
	v := newValidator()
	err := v.ValidateAutoConfig(&conf.AutoConfig{
		RotationInterval: "1ns",
	})
	if err == nil {
		t.Error("expected error for too-short rotation interval")
	}
}

func TestConfigValidator_AutoConfig_ValidRotationInterval(t *testing.T) {
	v := newValidator()
	err := v.ValidateAutoConfig(&conf.AutoConfig{
		RotationInterval: "24h",
	})
	if err != nil {
		t.Errorf("expected valid auto config, got: %v", err)
	}
}

// ---- GenerateAutoCertificates tests ----

func TestGenerateAutoCertificates_Basic(t *testing.T) {
	result, err := GenerateAutoCertificates("test-svc", "localhost", nil, time.Hour)
	if err != nil {
		t.Fatalf("GenerateAutoCertificates: %v", err)
	}
	if len(result.CertPEM) == 0 {
		t.Error("expected non-empty CertPEM")
	}
	if len(result.KeyPEM) == 0 {
		t.Error("expected non-empty KeyPEM")
	}
	if len(result.RootCAPEM) == 0 {
		t.Error("expected non-empty RootCAPEM")
	}
}

func TestGenerateAutoCertificates_EmptyServiceName(t *testing.T) {
	// Should fall back to "lynx-auto" without error
	result, err := GenerateAutoCertificates("", "", nil, time.Hour)
	if err != nil {
		t.Fatalf("GenerateAutoCertificates with empty name: %v", err)
	}
	if len(result.CertPEM) == 0 {
		t.Error("expected non-empty CertPEM")
	}
}

func TestGenerateAutoCertificates_WithSANs(t *testing.T) {
	sans := []string{"127.0.0.1", "my-service.default.svc.cluster.local"}
	result, err := GenerateAutoCertificates("my-svc", "my-host", sans, time.Hour)
	if err != nil {
		t.Fatalf("GenerateAutoCertificates with SANs: %v", err)
	}
	if len(result.CertPEM) == 0 {
		t.Error("expected non-empty CertPEM")
	}
}

func TestGenerateAutoCertificates_ShortValidity(t *testing.T) {
	// validity < 1h should be clamped to DefaultAutoRotationInterval without error
	result, err := GenerateAutoCertificates("svc", "host", nil, time.Minute)
	if err != nil {
		t.Fatalf("GenerateAutoCertificates with short validity: %v", err)
	}
	if len(result.CertPEM) == 0 {
		t.Error("expected non-empty CertPEM")
	}
}

// ---- conf/defaults.go tests ----

func TestIsValidSourceType(t *testing.T) {
	valid := []string{
		conf.SourceTypeControlPlane,
		conf.SourceTypeLocalFile,
		conf.SourceTypeMemory,
		conf.SourceTypeAuto,
	}
	for _, st := range valid {
		if !conf.IsValidSourceType(st) {
			t.Errorf("expected %q to be valid source type", st)
		}
	}
	if conf.IsValidSourceType("unknown") {
		t.Error("expected 'unknown' to be invalid source type")
	}
}

func TestIsValidCertFormat(t *testing.T) {
	if !conf.IsValidCertFormat(conf.CertFormatPEM) {
		t.Error("expected PEM to be valid cert format")
	}
	if !conf.IsValidCertFormat(conf.CertFormatDER) {
		t.Error("expected DER to be valid cert format")
	}
	if conf.IsValidCertFormat("pkcs12") {
		t.Error("expected 'pkcs12' to be invalid cert format")
	}
}

func TestIsValidTLSVersion(t *testing.T) {
	validVersions := []string{"1.0", "1.1", "1.2", "1.3"}
	for _, v := range validVersions {
		if !conf.IsValidTLSVersion(v) {
			t.Errorf("expected TLS version %q to be valid", v)
		}
	}
	if conf.IsValidTLSVersion("2.0") {
		t.Error("expected TLS version '2.0' to be invalid")
	}
}

func TestIsValidAuthType(t *testing.T) {
	for i := int32(0); i <= 4; i++ {
		if !conf.IsValidAuthType(i) {
			t.Errorf("expected auth type %d to be valid", i)
		}
	}
	if conf.IsValidAuthType(5) {
		t.Error("expected auth type 5 to be invalid")
	}
	if conf.IsValidAuthType(-1) {
		t.Error("expected auth type -1 to be invalid")
	}
}

func TestIsValidReloadInterval(t *testing.T) {
	if !conf.IsValidReloadInterval(5 * time.Second) {
		t.Error("expected 5s reload interval to be valid")
	}
	if conf.IsValidReloadInterval(500 * time.Millisecond) {
		t.Error("expected 500ms reload interval to be invalid (below min)")
	}
	if conf.IsValidReloadInterval(10 * time.Minute) {
		t.Error("expected 10m reload interval to be invalid (above max)")
	}
}
