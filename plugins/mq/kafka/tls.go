package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/go-lynx/lynx/plugins/mq/kafka/conf"
)

// buildTLSConfig builds tls.Config based on configuration
func buildTLSConfig(t *conf.TLS) (*tls.Config, error) {
	if t == nil || !t.Enabled {
		return nil, nil
	}

	cfg := &tls.Config{
		InsecureSkipVerify: t.InsecureSkipVerify,
	}
	if t.ServerName != "" {
		cfg.ServerName = t.ServerName
	}

	// Load CA
	if t.CaFile != "" {
		caPEM, err := os.ReadFile(t.CaFile)
		if err != nil {
			return nil, fmt.Errorf("read ca_file failed: %w", err)
		}
		pool := x509.NewCertPool()
		if ok := pool.AppendCertsFromPEM(caPEM); !ok {
			return nil, fmt.Errorf("append ca cert failed")
		}
		cfg.RootCAs = pool
	}

	// Load client certificate
	if t.CertFile != "" && t.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(t.CertFile, t.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client cert/key failed: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}

	return cfg, nil
}
