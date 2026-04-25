package tls

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-lynx/lynx/tls/conf"
)

func TestCertificateManager_AutoSource_Initialize(t *testing.T) {
	config := &conf.Tls{
		SourceType: conf.SourceTypeAuto,
	}
	cm := NewCertificateManagerWithAuto(config, nil)
	err := cm.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer cm.Stop()

	if !cm.IsInitialized() {
		t.Fatal("expected manager to be initialized")
	}
	cert := cm.GetCertificate()
	key := cm.GetPrivateKey()
	rootCA := cm.GetRootCACertificate()
	if len(cert) == 0 || len(key) == 0 || len(rootCA) == 0 {
		t.Fatal("expected non-empty certificate, key, and root CA")
	}
	// Parse server cert
	block, _ := pem.Decode(cert)
	if block == nil {
		t.Fatal("failed to decode cert PEM")
	}
	_, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	tlsConfig := cm.GetTLSConfig()
	if tlsConfig == nil {
		t.Fatal("expected non-nil TLS config")
	}
	// Use GetCertificate for rotation; cert is fetched per handshake
	if tlsConfig.GetCertificate == nil {
		t.Fatal("expected GetCertificate to be set for dynamic cert (rotation)")
	}
	// Verify GetCertificate returns a valid cert
	dynamicCert, err := tlsConfig.GetCertificate(nil)
	if err != nil || dynamicCert == nil {
		t.Fatalf("GetCertificate should return current cert: err=%v cert=%v", err, dynamicCert)
	}
}

func TestCertificateManager_AutoSource_WithAutoConfig(t *testing.T) {
	config := &conf.Tls{
		SourceType: conf.SourceTypeAuto,
	}
	autoConfig := &conf.AutoConfig{
		ServiceName:      "test-service",
		Hostname:         "testhost",
		RotationInterval: "2h",
		CertValidity:     "2h",
		SANs:             []string{"localhost"},
	}
	cm := NewCertificateManagerWithAuto(config, autoConfig)
	err := cm.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer cm.Stop()

	cert := cm.GetCertificate()
	block, _ := pem.Decode(cert)
	if block == nil {
		t.Fatal("failed to decode cert PEM")
	}
	c, _ := x509.ParseCertificate(block.Bytes)
	if c.Subject.CommonName != "test-service" {
		t.Errorf("expected CN test-service, got %s", c.Subject.CommonName)
	}
}

func TestCertificateManager_AutoSource_ReloadRotatesCert(t *testing.T) {
	config := &conf.Tls{
		SourceType: conf.SourceTypeAuto,
	}
	cm := NewCertificateManagerWithAuto(config, nil)
	err := cm.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer cm.Stop()

	rootAfterInit := cm.GetRootCACertificate()
	cert1 := cm.GetCertificate()
	// Reload (simulate rotation)
	err = cm.reloadCertificates()
	if err != nil {
		t.Fatalf("reloadCertificates failed: %v", err)
	}
	cert2 := cm.GetCertificate()
	// After rotation, cert bytes should be different (new serial)
	if string(cert1) == string(cert2) {
		t.Error("expected certificate to change after reload")
	}
	// Both should be valid PEM
	block1, _ := pem.Decode(cert1)
	block2, _ := pem.Decode(cert2)
	if block1 == nil || block2 == nil {
		t.Fatal("expected valid PEM after reload")
	}
	c1, _ := x509.ParseCertificate(block1.Bytes)
	c2, _ := x509.ParseCertificate(block2.Bytes)
	if c1.SerialNumber.Cmp(c2.SerialNumber) == 0 {
		t.Error("expected different serial numbers after rotation")
	}
	if string(rootAfterInit) != string(cm.GetRootCACertificate()) {
		t.Error("fixed root: GetRootCACertificate should stay byte-identical across leaf rotation")
	}
}

func TestCertificateManager_ReloadUnsupportedSource(t *testing.T) {
	config := &conf.Tls{
		SourceType: conf.SourceTypeControlPlane,
		FileName:   "dummy",
	}
	cm := NewCertificateManager(config)
	// Initialize would fail without real control plane; we only test reload path
	err := cm.reloadCertificates()
	if err == nil {
		t.Fatal("expected reload to fail for control_plane source")
	}
}

func TestCertificateManager_StopStopsAutoRotation(t *testing.T) {
	config := &conf.Tls{
		SourceType: conf.SourceTypeAuto,
	}
	autoConfig := &conf.AutoConfig{
		RotationInterval: "1h", // valid range; test only verifies Stop() stops the goroutine
	}
	cm := NewCertificateManagerWithAuto(config, autoConfig)
	err := cm.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	// Stop should not panic and should stop the rotation goroutine
	cm.Stop()
	cm.Stop() // idempotent
	if cm.IsInitialized() {
		t.Error("expected not initialized after Stop")
	}
}

func TestCertificateManager_AutoSource_SharedCA_File(t *testing.T) {
	// Create shared CA (as if created once for the mesh)
	caCertPEM, caKeyPEM, err := GenerateCAOnly()
	if err != nil {
		t.Fatalf("GenerateCAOnly failed: %v", err)
	}
	dir := t.TempDir()
	caCertPath := filepath.Join(dir, "ca.pem")
	caKeyPath := filepath.Join(dir, "ca-key.pem")
	if err := os.WriteFile(caCertPath, caCertPEM, 0600); err != nil {
		t.Fatalf("write CA cert: %v", err)
	}
	if err := os.WriteFile(caKeyPath, caKeyPEM, 0600); err != nil {
		t.Fatalf("write CA key: %v", err)
	}
	config := &conf.Tls{SourceType: conf.SourceTypeAuto}
	autoConfig := &conf.AutoConfig{
		ServiceName:      "svc-a",
		Hostname:         "host-a",
		RotationInterval: "24h",
		CertValidity:     "24h",
		SharedCA: &conf.SharedCAConfig{
			From:     conf.SharedCAFromFile,
			CertFile: caCertPath,
			KeyFile:  caKeyPath,
		},
	}
	cm := NewCertificateManagerWithAuto(config, autoConfig)
	err = cm.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer cm.Stop()
	// Root CA should be the shared CA (so A can use GetRootCACertificate for client verification)
	rootCA := cm.GetRootCACertificate()
	if string(rootCA) != string(caCertPEM) {
		t.Error("expected GetRootCACertificate to return shared CA PEM")
	}
	// Server cert should be signed by shared CA
	block, _ := pem.Decode(cm.GetCertificate())
	if block == nil {
		t.Fatal("failed to decode server cert")
	}
	serverCert, _ := x509.ParseCertificate(block.Bytes)
	caBlock, _ := pem.Decode(caCertPEM)
	caCert, _ := x509.ParseCertificate(caBlock.Bytes)
	if err := serverCert.CheckSignatureFrom(caCert); err != nil {
		t.Errorf("server cert should be signed by shared CA: %v", err)
	}
	if serverCert.Subject.CommonName != "svc-a" {
		t.Errorf("expected CN svc-a, got %s", serverCert.Subject.CommonName)
	}
}

func TestCertificateManager_ReturnsDefensiveCopies(t *testing.T) {
	config := &conf.Tls{SourceType: conf.SourceTypeAuto}
	cm := NewCertificateManagerWithAuto(config, nil)
	if err := cm.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer cm.Stop()

	cert := cm.GetCertificate()
	key := cm.GetPrivateKey()
	rootCA := cm.GetRootCACertificate()
	if len(cert) == 0 || len(key) == 0 || len(rootCA) == 0 {
		t.Fatal("expected certificate material")
	}
	cert[0] ^= 0xff
	key[0] ^= 0xff
	rootCA[0] ^= 0xff

	if _, err := tls.X509KeyPair(cm.GetCertificate(), cm.GetPrivateKey()); err != nil {
		t.Fatalf("internal certificate material should not be mutable by callers: %v", err)
	}
	if string(cm.GetCertificate()) == string(cert) {
		t.Fatal("expected GetCertificate to return a copy")
	}
	if string(cm.GetPrivateKey()) == string(key) {
		t.Fatal("expected GetPrivateKey to return a copy")
	}
	if string(cm.GetRootCACertificate()) == string(rootCA) {
		t.Fatal("expected GetRootCACertificate to return a copy")
	}
}

func TestCertificateManager_TLSConfigUsesDefensiveSnapshot(t *testing.T) {
	config := &conf.Tls{SourceType: conf.SourceTypeAuto}
	cm := NewCertificateManagerWithAuto(config, nil)
	if err := cm.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer cm.Stop()

	cfg := cm.GetTLSConfig()
	if cfg == nil || cfg.GetCertificate == nil {
		t.Fatal("expected TLS config with GetCertificate")
	}
	first, err := cfg.GetCertificate(nil)
	if err != nil {
		t.Fatalf("GetCertificate failed: %v", err)
	}
	if len(first.Certificate) == 0 || len(first.Certificate[0]) == 0 {
		t.Fatal("expected parsed certificate chain")
	}
	first.Certificate[0][0] ^= 0xff

	second, err := cfg.GetCertificate(nil)
	if err != nil {
		t.Fatalf("GetCertificate failed after caller mutation: %v", err)
	}
	if second.Certificate[0][0] == first.Certificate[0][0] {
		t.Fatal("expected TLS certificate callback to return an isolated snapshot")
	}

	cfg.MinVersion = tls.VersionTLS10
	if got := cm.GetTLSConfig().MinVersion; got != tls.VersionTLS12 {
		t.Fatalf("external TLS config mutation leaked into manager: got %x", got)
	}
}

func TestNewCertificateManagerWithAuto_SetsAutoConfig(t *testing.T) {
	config := &conf.Tls{SourceType: conf.SourceTypeAuto}
	ac := &conf.AutoConfig{ServiceName: "x"}
	cm := NewCertificateManagerWithAuto(config, ac)
	if cm.autoConfig != ac {
		t.Error("expected autoConfig to be set")
	}
	err := cm.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	cm.Stop()
}
