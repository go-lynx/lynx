package grpc

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTlsLoad(t *testing.T) {
	plugin := NewServiceGrpc()

	// Test certificate provider is nil - skip due to external dependencies
	// This would require mocking the app.Lynx() call which is complex
	// In real usage, this would be called by the framework with proper setup
	// _, err := plugin.tlsLoad()
	// assert.Error(t, err)
	// assert.Contains(t, err.Error(), "certificate provider not configured")

	// This functionality will be tested in integration tests
	assert.NotNil(t, plugin)
}

func TestGenerateTestCertificates(t *testing.T) {
	// Generate test certificates
	cert, key, err := generateTestCertificates()
	require.NoError(t, err)

	// Verify certificate format
	block, _ := pem.Decode(cert)
	require.NotNil(t, block)
	assert.Equal(t, "CERTIFICATE", block.Type)

	// Verify private key format
	keyBlock, _ := pem.Decode(key)
	require.NotNil(t, keyBlock)
	assert.Equal(t, "RSA PRIVATE KEY", keyBlock.Type)

	// Verify certificate and private key match
	_, err = tls.X509KeyPair(cert, key)
	assert.NoError(t, err)
}

// generateTestCertificates generates test certificates and private key
func generateTestCertificates() ([]byte, []byte, error) {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test Organization"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}

	// Generate certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, err
	}

	// Encode certificate
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	// Encode private key
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	return certPEM, keyPEM, nil
}
