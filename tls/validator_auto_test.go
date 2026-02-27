package tls

import (
	"testing"

	"github.com/go-lynx/lynx/tls/conf"
)

func TestValidator_AutoSource(t *testing.T) {
	v := NewConfigValidator()
	config := &conf.Tls{
		SourceType: conf.SourceTypeAuto,
	}
	err := v.Validate(config)
	if err != nil {
		t.Fatalf("Validate with auto source should succeed: %v", err)
	}
	err = v.ValidateCompleteConfig(config)
	if err != nil {
		t.Fatalf("ValidateCompleteConfig with auto source should succeed: %v", err)
	}
}

func TestValidateAutoConfig_Nil(t *testing.T) {
	v := NewConfigValidator()
	err := v.ValidateAutoConfig(nil)
	if err != nil {
		t.Fatalf("ValidateAutoConfig(nil) should succeed: %v", err)
	}
}

func TestValidateAutoConfig_ValidInterval(t *testing.T) {
	v := NewConfigValidator()
	ac := &conf.AutoConfig{
		RotationInterval: "24h",
	}
	err := v.ValidateAutoConfig(ac)
	if err != nil {
		t.Fatalf("ValidateAutoConfig with valid interval should succeed: %v", err)
	}
}

func TestValidateAutoConfig_InvalidInterval(t *testing.T) {
	v := NewConfigValidator()
	ac := &conf.AutoConfig{
		RotationInterval: "invalid",
	}
	err := v.ValidateAutoConfig(ac)
	if err == nil {
		t.Fatal("ValidateAutoConfig with invalid interval should fail")
	}
}

func TestValidateAutoConfig_IntervalOutOfRange(t *testing.T) {
	v := NewConfigValidator()
	ac := &conf.AutoConfig{
		RotationInterval: "1m", // less than MinAutoRotationInterval (1h)
	}
	err := v.ValidateAutoConfig(ac)
	if err == nil {
		t.Fatal("ValidateAutoConfig with too short interval should fail")
	}
	ac.RotationInterval = "9999h" // over MaxAutoRotationInterval (168h)
	err = v.ValidateAutoConfig(ac)
	if err == nil {
		t.Fatal("ValidateAutoConfig with too long interval should fail")
	}
}

func TestValidateSharedCAConfig_Nil(t *testing.T) {
	v := NewConfigValidator()
	if err := v.ValidateSharedCAConfig(nil); err != nil {
		t.Fatalf("ValidateSharedCAConfig(nil) should succeed: %v", err)
	}
}

func TestValidateSharedCAConfig_File_MissingFields(t *testing.T) {
	v := NewConfigValidator()
	shared := &conf.SharedCAConfig{From: conf.SharedCAFromFile}
	if err := v.ValidateSharedCAConfig(shared); err == nil {
		t.Fatal("expected failure when cert_file/key_file missing")
	}
	shared.CertFile = "/tmp/ca.pem"
	if err := v.ValidateSharedCAConfig(shared); err == nil {
		t.Fatal("expected failure when key_file missing")
	}
}

func TestValidateSharedCAConfig_ControlPlane_MissingName(t *testing.T) {
	v := NewConfigValidator()
	shared := &conf.SharedCAConfig{From: conf.SharedCAFromControlPlane}
	if err := v.ValidateSharedCAConfig(shared); err == nil {
		t.Fatal("expected failure when config_name missing")
	}
}

func TestValidateSharedCAConfig_InvalidFrom(t *testing.T) {
	v := NewConfigValidator()
	shared := &conf.SharedCAConfig{From: "invalid"}
	if err := v.ValidateSharedCAConfig(shared); err == nil {
		t.Fatal("expected failure for invalid from")
	}
}
