package tls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"strings"
	"time"

	"github.com/go-lynx/lynx/tls/conf"
)

// CertGenResult holds PEM-encoded certificate and key bytes.
type CertGenResult struct {
	CertPEM   []byte
	KeyPEM    []byte
	RootCAPEM []byte
}

// GenerateAutoCertificates generates a root CA and a server certificate signed by it.
// Server cert uses serviceName as CN and includes hostname + SANs in SubjectAlternativeNames.
// Validity is the duration the server cert is valid for; CA is valid for 10 years.
func GenerateAutoCertificates(serviceName, hostname string, sans []string, validity time.Duration) (*CertGenResult, error) {
	r, _, _, err := generateAutoStack(serviceName, hostname, sans, validity)
	return r, err
}

// generateAutoStack creates a new in-process CA and a server certificate, and returns the CA PEM pair
// so the caller can reissue leaf certificates without rotating the root (fixed-root rotation).
func generateAutoStack(serviceName, hostname string, sans []string, validity time.Duration) (*CertGenResult, []byte, []byte, error) {
	if serviceName == "" {
		serviceName = "lynx-auto"
	}
	if hostname == "" {
		var e error
		hostname, e = os.Hostname()
		if e != nil {
			hostname = "localhost"
		}
	}
	if validity < time.Hour {
		validity = conf.DefaultAutoRotationInterval
	}

	caPEM, caKeyPEM, caCert, caKey, err := generateCA()
	if err != nil {
		return nil, nil, nil, err
	}

	dnsNames, ipAddrs := buildServerSAN(serviceName, hostname, sans)

	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generate server key: %w", err)
	}
	serverSerial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generate server serial: %w", err)
	}
	serverTemplate := &x509.Certificate{
		SerialNumber: serverSerial,
		Subject: pkix.Name{
			Organization: []string{"Lynx"},
			CommonName:   serviceName,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(validity),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    dnsNames,
		IPAddresses: ipAddrs,
	}
	serverCertDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create server cert: %w", err)
	}
	serverPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverCertDER})
	serverKeyDER, err := x509.MarshalECPrivateKey(serverKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal server key: %w", err)
	}
	serverKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: serverKeyDER})

	return &CertGenResult{
		CertPEM:   serverPEM,
		KeyPEM:    serverKeyPEM,
		RootCAPEM: caPEM,
	}, caPEM, caKeyPEM, nil
}

// GenerateCAOnly generates only a root CA (cert + key PEM). Used for creating the shared mesh CA once.
func GenerateCAOnly() (caCertPEM, caKeyPEM []byte, err error) {
	caPEM, caKeyPEM, _, _, err := generateCA()
	return caPEM, caKeyPEM, err
}

// generateCA creates a new ECDSA P-256 CA and returns PEMs and parsed cert/key for signing.
func generateCA() (caCertPEM, caKeyPEM []byte, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, err error) {
	caKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("generate CA key: %w", err)
	}
	caSerial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("generate CA serial: %w", err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber: caSerial,
		Subject: pkix.Name{
			Organization: []string{"Lynx Auto CA"},
			CommonName:   "lynx-auto-ca",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("create CA cert: %w", err)
	}
	caCert, err = x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("parse CA cert: %w", err)
	}
	caCertPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})
	caKeyDER, err := x509.MarshalECPrivateKey(caKey)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("marshal CA key: %w", err)
	}
	caKeyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: caKeyDER})
	return caCertPEM, caKeyPEM, caCert, caKey, nil
}

// GenerateServerCertFromCA generates only a server certificate (and key) signed by the given CA.
// caCertPEM and caKeyPEM are the shared root CA certificate and private key (PEM).
// The returned RootCAPEM is the same as caCertPEM so the mesh can use one CA for verification.
// Supports ECDSA and RSA CA keys (PEM blocks: EC PRIVATE KEY, PRIVATE KEY, or RSA PRIVATE KEY).
func GenerateServerCertFromCA(caCertPEM, caKeyPEM []byte, serviceName, hostname string, sans []string, validity time.Duration) (*CertGenResult, error) {
	if len(caCertPEM) == 0 || len(caKeyPEM) == 0 {
		return nil, fmt.Errorf("CA cert and key PEM are required")
	}
	caCert, caKey, err := parseCAFromPEM(caCertPEM, caKeyPEM)
	if err != nil {
		return nil, err
	}
	if serviceName == "" {
		serviceName = "lynx-auto"
	}
	if hostname == "" {
		var err error
		hostname, err = os.Hostname()
		if err != nil {
			hostname = "localhost"
		}
	}
	if validity < time.Hour {
		validity = conf.DefaultAutoRotationInterval
	}
	dnsNames, ipAddrs := buildServerSAN(serviceName, hostname, sans)

	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate server key: %w", err)
	}
	serverSerial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate server serial: %w", err)
	}
	serverTemplate := &x509.Certificate{
		SerialNumber: serverSerial,
		Subject: pkix.Name{
			Organization: []string{"Lynx"},
			CommonName:   serviceName,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(validity),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    dnsNames,
		IPAddresses: ipAddrs,
	}
	serverCertDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("create server cert from CA: %w", err)
	}
	serverPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverCertDER})
	serverKeyDER, err := x509.MarshalECPrivateKey(serverKey)
	if err != nil {
		return nil, fmt.Errorf("marshal server key: %w", err)
	}
	serverKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: serverKeyDER})

	return &CertGenResult{
		CertPEM:   serverPEM,
		KeyPEM:    serverKeyPEM,
		RootCAPEM: caCertPEM,
	}, nil
}

// parseCAFromPEM decodes CA cert and key from PEM; supports EC and RSA private keys.
func parseCAFromPEM(caCertPEM, caKeyPEM []byte) (*x509.Certificate, interface{}, error) {
	block, _ := pem.Decode(caCertPEM)
	if block == nil {
		return nil, nil, fmt.Errorf("failed to decode CA cert PEM")
	}
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse CA cert: %w", err)
	}
	keyBlock, _ := pem.Decode(caKeyPEM)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode CA key PEM")
	}
	var caKey interface{}
	switch keyBlock.Type {
	case "EC PRIVATE KEY":
		caKey, err = x509.ParseECPrivateKey(keyBlock.Bytes)
	case "PRIVATE KEY":
		caKey, err = x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	default:
		return nil, nil, fmt.Errorf("unsupported CA key PEM type: %s", keyBlock.Type)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("parse CA key: %w", err)
	}
	return caCert, caKey, nil
}

// buildServerSAN returns DNS names and IP addresses for SAN from serviceName, hostname, sans.
func buildServerSAN(serviceName, hostname string, sans []string) (dnsNames []string, ipAddrs []net.IP) {
	names := make(map[string]struct{})
	names[hostname] = struct{}{}
	names[serviceName] = struct{}{}
	for _, s := range sans {
		s = strings.TrimSpace(s)
		if s != "" {
			names[s] = struct{}{}
		}
	}
	for n := range names {
		if ip := net.ParseIP(n); ip != nil {
			ipAddrs = append(ipAddrs, ip)
		} else {
			dnsNames = append(dnsNames, n)
		}
	}
	if len(dnsNames) == 0 {
		dnsNames = append(dnsNames, "localhost")
	}
	has127 := false
	for _, ip := range ipAddrs {
		if ip.Equal(net.IPv4(127, 0, 0, 1)) || ip.Equal(net.IPv6loopback) {
			has127 = true
			break
		}
	}
	if !has127 {
		ipAddrs = append(ipAddrs, net.IPv4(127, 0, 0, 1))
	}
	return dnsNames, ipAddrs
}

// GenerateAutoCertificatesFromConfig uses AutoConfig to build parameters and calls GenerateAutoCertificates.
// When cfg.SharedCA is set, the caller (CertificateManager) loads the CA and uses GenerateServerCertFromCA instead.
// If cfg is nil, fallbacks and defaults are used.
func GenerateAutoCertificatesFromConfig(cfg *conf.AutoConfig, fallbackServiceName, fallbackHostname string) (*CertGenResult, error) {
	r, _, _, err := generateAutoStackFromConfig(cfg, fallbackServiceName, fallbackHostname)
	return r, err
}

// generateAutoStackFromConfig builds parameters from AutoConfig and issues a new in-process CA plus server cert.
// Returns CA PEM material for CertificateManager to retain for fixed-root leaf rotation.
func generateAutoStackFromConfig(cfg *conf.AutoConfig, fallbackServiceName, fallbackHostname string) (*CertGenResult, []byte, []byte, error) {
	var serviceName, hostname string
	var sans []string
	var validity time.Duration
	if cfg != nil {
		serviceName = cfg.ServiceName
		hostname = cfg.Hostname
		sans = cfg.SANs
		validity = cfg.ParseAutoCertValidity()
	}
	if serviceName == "" {
		serviceName = fallbackServiceName
	}
	if serviceName == "" {
		serviceName = "lynx-auto"
	}
	if hostname == "" {
		hostname = fallbackHostname
	}
	if validity == 0 {
		validity = conf.DefaultAutoRotationInterval
	}
	return generateAutoStack(serviceName, hostname, sans, validity)
}
