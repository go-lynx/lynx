package conf

import (
	"testing"
	"time"
)

func TestSourceTypeAuto_IsValidSourceType(t *testing.T) {
	if !IsValidSourceType(SourceTypeAuto) {
		t.Error("SourceTypeAuto should be valid")
	}
}

func TestAutoConfig_ParseAutoRotationInterval_Empty(t *testing.T) {
	var ac *AutoConfig
	d := ac.ParseAutoRotationInterval()
	if d != DefaultAutoRotationInterval {
		t.Errorf("expected default %v, got %v", DefaultAutoRotationInterval, d)
	}
	ac = &AutoConfig{}
	d = ac.ParseAutoRotationInterval()
	if d != DefaultAutoRotationInterval {
		t.Errorf("expected default %v, got %v", DefaultAutoRotationInterval, d)
	}
}

func TestAutoConfig_ParseAutoRotationInterval_Valid(t *testing.T) {
	ac := &AutoConfig{RotationInterval: "12h"}
	d := ac.ParseAutoRotationInterval()
	if d != 12*time.Hour {
		t.Errorf("expected 12h, got %v", d)
	}
}

func TestAutoConfig_ParseAutoRotationInterval_InvalidFallsBackToDefault(t *testing.T) {
	ac := &AutoConfig{RotationInterval: "invalid"}
	d := ac.ParseAutoRotationInterval()
	if d != DefaultAutoRotationInterval {
		t.Errorf("expected default on invalid, got %v", d)
	}
}

func TestAutoConfig_ParseAutoRotationInterval_OutOfRangeFallsBackToDefault(t *testing.T) {
	ac := &AutoConfig{RotationInterval: "30m"} // below min
	d := ac.ParseAutoRotationInterval()
	if d != DefaultAutoRotationInterval {
		t.Errorf("expected default when below min, got %v", d)
	}
}

func TestAutoConfig_ParseAutoCertValidity_EmptyUsesRotationInterval(t *testing.T) {
	ac := &AutoConfig{RotationInterval: "6h"}
	d := ac.ParseAutoCertValidity()
	if d != 6*time.Hour {
		t.Errorf("expected 6h, got %v", d)
	}
}

func TestAutoConfig_ParseAutoCertValidity_Explicit(t *testing.T) {
	ac := &AutoConfig{RotationInterval: "24h", CertValidity: "48h"}
	d := ac.ParseAutoCertValidity()
	if d != 48*time.Hour {
		t.Errorf("expected 48h, got %v", d)
	}
}
