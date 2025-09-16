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

	"github.com/go-lynx/lynx/plugins/service/grpc/conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCertProvider is a mock implementation for testing
type mockCertProvider struct {
	certificate []byte
	privateKey  []byte
	rootCA      []byte
}

func (m *mockCertProvider) GetCertificate() ([]byte, error) {
	return m.certificate, nil
}

func (m *mockCertProvider) GetPrivateKey() ([]byte, error) {
	return m.privateKey, nil
}

func (m *mockCertProvider) GetRootCA() ([]byte, error) {
	return m.rootCA, nil
}

// generateTestCertificates creates test certificates for testing
func generateTestCertificates() ([]byte, []byte, error) {
	// Generate private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"Test"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}

	// Encode certificate
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	// Encode private key
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})

	return certPEM, keyPEM, nil
}

func TestTlsLoad(t *testing.T) {
	plugin := NewGrpcService()

	// Test with nil certificate provider
	plugin.conf = &conf.Service{
		TlsEnable: true,
	}

	// Test with nil certificate provider (default behavior)
	// Note: getCertProvider is a method, not a field, so we test the default behavior

	_, err := plugin.tlsLoad()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "certificate provider not configured")

	// Test with mock certificate provider
	cert, key, err := generateTestCertificates()
	require.NoError(t, err)

	mockCertProvider := &mockCertProvider{
		certificate: cert,
		privateKey:  key,
		rootCA:      cert, // Use same cert as root CA for testing
	}

	// Test with mock certificate provider
	// Note: We'll test the tlsLoad method with proper configuration instead
	plugin.certProvider = func() interface{} {
		return mockCertProvider
	}

	tlsOption, err := plugin.tlsLoad()
	assert.NoError(t, err)
	assert.NotNil(t, tlsOption)
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
