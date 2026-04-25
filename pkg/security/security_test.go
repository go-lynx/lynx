package security

import "testing"

func TestValidateTLSProductionPolicy(t *testing.T) {
	t.Setenv("LYNX_ENV", "production")

	if err := ValidateTLSProductionPolicy("redis", true, true); err == nil {
		t.Fatal("expected production insecure TLS to be rejected")
	}
	if err := ValidateTLSProductionPolicy("redis", true, false); err != nil {
		t.Fatalf("expected secure TLS to pass: %v", err)
	}
	if err := ValidateTLSProductionPolicy("redis", false, true); err != nil {
		t.Fatalf("expected disabled TLS to pass: %v", err)
	}
}

func TestValidateTLSProductionPolicyAllowsLocalInsecureTLS(t *testing.T) {
	t.Setenv("LYNX_ENV", "local")

	if err := ValidateTLSProductionPolicy("redis", true, true); err != nil {
		t.Fatalf("expected local insecure TLS to pass: %v", err)
	}
}
