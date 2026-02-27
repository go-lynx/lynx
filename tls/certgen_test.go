package tls

import (
	"crypto/x509"
	"encoding/pem"
	"net"
	"testing"
	"time"

	"github.com/go-lynx/lynx/tls/conf"
)

func TestGenerateAutoCertificates_DefaultValues(t *testing.T) {
	result, err := GenerateAutoCertificates("", "", nil, 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateAutoCertificates failed: %v", err)
	}
	if len(result.CertPEM) == 0 || len(result.KeyPEM) == 0 || len(result.RootCAPEM) == 0 {
		t.Fatal("expected non-empty PEM outputs")
	}
	// Parse and verify server cert
	block, _ := pem.Decode(result.CertPEM)
	if block == nil {
		t.Fatal("failed to decode server cert PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse server cert: %v", err)
	}
	if cert.Subject.CommonName != "lynx-auto" {
		t.Errorf("expected CN lynx-auto, got %s", cert.Subject.CommonName)
	}
	// Root CA should parse
	caBlock, _ := pem.Decode(result.RootCAPEM)
	if caBlock == nil {
		t.Fatal("failed to decode CA PEM")
	}
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		t.Fatalf("parse CA cert: %v", err)
	}
	if !caCert.IsCA {
		t.Error("expected CA cert to have IsCA true")
	}
	// Server cert should be valid for ~24h
	if cert.NotAfter.Sub(cert.NotBefore) < 23*time.Hour || cert.NotAfter.Sub(cert.NotBefore) > 25*time.Hour {
		t.Errorf("expected ~24h validity, got %v", cert.NotAfter.Sub(cert.NotBefore))
	}
}

func TestGenerateAutoCertificates_WithSANs(t *testing.T) {
	result, err := GenerateAutoCertificates("my-svc", "myhost", []string{"localhost", "127.0.0.1", "extra.local"}, 2*time.Hour)
	if err != nil {
		t.Fatalf("GenerateAutoCertificates failed: %v", err)
	}
	block, _ := pem.Decode(result.CertPEM)
	if block == nil {
		t.Fatal("failed to decode server cert PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse server cert: %v", err)
	}
	if cert.Subject.CommonName != "my-svc" {
		t.Errorf("expected CN my-svc, got %s", cert.Subject.CommonName)
	}
	// Check DNS names contain expected
	hasLocalhost := false
	hasExtra := false
	for _, d := range cert.DNSNames {
		if d == "localhost" {
			hasLocalhost = true
		}
		if d == "extra.local" {
			hasExtra = true
		}
	}
	if !hasLocalhost {
		t.Error("expected localhost in DNSNames")
	}
	if !hasExtra {
		t.Error("expected extra.local in DNSNames")
	}
	// Should have 127.0.0.1 in IPAddresses
	has127 := false
	for _, ip := range cert.IPAddresses {
		if ip.Equal(net.IPv4(127, 0, 0, 1)) {
			has127 = true
			break
		}
	}
	if !has127 {
		t.Error("expected 127.0.0.1 in IPAddresses")
	}
}

func TestGenerateAutoCertificatesFromConfig_NilConfig(t *testing.T) {
	result, err := GenerateAutoCertificatesFromConfig(nil, "fallback-svc", "fallback-host")
	if err != nil {
		t.Fatalf("GenerateAutoCertificatesFromConfig failed: %v", err)
	}
	if len(result.CertPEM) == 0 {
		t.Fatal("expected non-empty cert")
	}
	block, _ := pem.Decode(result.CertPEM)
	if block == nil {
		t.Fatal("failed to decode cert PEM")
	}
	cert, _ := x509.ParseCertificate(block.Bytes)
	if cert.Subject.CommonName != "fallback-svc" {
		t.Errorf("expected CN fallback-svc, got %s", cert.Subject.CommonName)
	}
}

func TestGenerateAutoCertificatesFromConfig_WithConfig(t *testing.T) {
	cfg := &conf.AutoConfig{
		ServiceName:  "configured-svc",
		Hostname:     "configured-host",
		SANs:         []string{"a.local"},
		CertValidity: "48h",
	}
	result, err := GenerateAutoCertificatesFromConfig(cfg, "fallback", "fallback")
	if err != nil {
		t.Fatalf("GenerateAutoCertificatesFromConfig failed: %v", err)
	}
	block, _ := pem.Decode(result.CertPEM)
	if block == nil {
		t.Fatal("failed to decode cert PEM")
	}
	cert, _ := x509.ParseCertificate(block.Bytes)
	if cert.Subject.CommonName != "configured-svc" {
		t.Errorf("expected CN configured-svc, got %s", cert.Subject.CommonName)
	}
	// Validity should be ~48h
	d := cert.NotAfter.Sub(cert.NotBefore)
	if d < 47*time.Hour || d > 49*time.Hour {
		t.Errorf("expected ~48h validity, got %v", d)
	}
}

func TestGenerateServerCertFromCA(t *testing.T) {
	// Create a shared CA once (like mesh bootstrap)
	caCertPEM, caKeyPEM, err := GenerateCAOnly()
	if err != nil {
		t.Fatalf("GenerateCAOnly failed: %v", err)
	}
	// Service B gets a server cert signed by the shared CA
	resultB, err := GenerateServerCertFromCA(caCertPEM, caKeyPEM, "svc-b", "host-b", []string{"localhost"}, 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateServerCertFromCA failed: %v", err)
	}
	if len(resultB.RootCAPEM) == 0 || string(resultB.RootCAPEM) != string(caCertPEM) {
		t.Error("expected RootCAPEM to equal input CA cert for mesh verification")
	}
	block, _ := pem.Decode(resultB.CertPEM)
	if block == nil {
		t.Fatal("failed to decode server cert PEM")
	}
	certB, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse server cert: %v", err)
	}
	if certB.Subject.CommonName != "svc-b" {
		t.Errorf("expected CN svc-b, got %s", certB.Subject.CommonName)
	}
	// Verify cert B is signed by the CA
	caBlock, _ := pem.Decode(caCertPEM)
	caCert, _ := x509.ParseCertificate(caBlock.Bytes)
	if err := certB.CheckSignatureFrom(caCert); err != nil {
		t.Errorf("server cert should be signed by CA: %v", err)
	}
}

func TestGenerateCAOnly(t *testing.T) {
	caCertPEM, caKeyPEM, err := GenerateCAOnly()
	if err != nil {
		t.Fatalf("GenerateCAOnly failed: %v", err)
	}
	if len(caCertPEM) == 0 || len(caKeyPEM) == 0 {
		t.Fatal("expected non-empty CA cert and key")
	}
	block, _ := pem.Decode(caCertPEM)
	if block == nil {
		t.Fatal("failed to decode CA cert PEM")
	}
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse CA cert: %v", err)
	}
	if !caCert.IsCA {
		t.Error("expected IsCA true")
	}
}

func TestGenerateAutoCertificates_ShortValidityClamped(t *testing.T) {
	// validity < 1h should be clamped to DefaultAutoRotationInterval (24h)
	result, err := GenerateAutoCertificates("svc", "host", nil, 30*time.Minute)
	if err != nil {
		t.Fatalf("GenerateAutoCertificates failed: %v", err)
	}
	block, _ := pem.Decode(result.CertPEM)
	if block == nil {
		t.Fatal("failed to decode cert PEM")
	}
	cert, _ := x509.ParseCertificate(block.Bytes)
	d := cert.NotAfter.Sub(cert.NotBefore)
	if d < 23*time.Hour {
		t.Errorf("expected validity clamped to at least 23h, got %v", d)
	}
}
