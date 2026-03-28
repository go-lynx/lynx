package subscribe

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-lynx/lynx/conf"
)

func TestNewGrpcSubscribe_OptionsApplied(t *testing.T) {
	var routerCalled bool
	gs := NewGrpcSubscribe(
		WithServiceName("demo.svc"),
		WithDiscovery(nil),
		EnableTls(),
		WithRootCAFileName("ca.pem"),
		WithRootCAFileGroup("grp"),
		Required(),
		WithNodeRouterFactory(func(service string) selector.NodeFilter {
			routerCalled = true
			if service != "demo.svc" {
				t.Errorf("router factory service: got %q", service)
			}
			return nil
		}),
		WithConfigProvider(func(name, group string) (config.Source, error) {
			return nil, errors.New("unused in this test")
		}),
		WithDefaultRootCA(func() []byte { return nil }),
	)
	if gs.svcName != "demo.svc" || !gs.tls || gs.caName != "ca.pem" || gs.caGroup != "grp" || !gs.required {
		t.Fatalf("svcName=%q tls=%v caName=%q caGroup=%q required=%v",
			gs.svcName, gs.tls, gs.caName, gs.caGroup, gs.required)
	}
	_ = gs.nodeFilter()
	if !routerCalled {
		t.Fatal("expected node router factory to be invoked by nodeFilter")
	}
}

func TestGrpcSubscribe_Subscribe_EmptyServiceName(t *testing.T) {
	g := NewGrpcSubscribe(WithServiceName(""))
	if g.Subscribe() != nil {
		t.Fatal("expected nil conn when service name is empty")
	}
}

func TestGrpcSubscribe_nodeFilter_WithoutFactory(t *testing.T) {
	g := NewGrpcSubscribe(WithServiceName("x"))
	if g.nodeFilter() != nil {
		t.Fatal("expected nil NodeFilter when routerFactory is unset")
	}
}

func TestGrpcSubscribe_buildClientTLSConfig_TLSOff(t *testing.T) {
	g := NewGrpcSubscribe(WithServiceName("svc"))
	cfg, err := g.buildClientTLSConfig()
	if err != nil || cfg != nil {
		t.Fatalf("tls off: want nil, nil; got cfg=%v err=%v", cfg, err)
	}
}

func TestGrpcSubscribe_buildClientTLSConfig_TLSErrors(t *testing.T) {
	t.Run("caName_set_configProvider_nil", func(t *testing.T) {
		g := NewGrpcSubscribe(WithServiceName("upstream"), EnableTls(), WithRootCAFileName("ca.json"))
		_, err := g.buildClientTLSConfig()
		if err == nil || !strings.Contains(err.Error(), "configProvider is nil") {
			t.Fatalf("got err=%v", err)
		}
	})
	t.Run("caName_empty_defaultRootCA_nil", func(t *testing.T) {
		g := NewGrpcSubscribe(WithServiceName("upstream"), EnableTls())
		_, err := g.buildClientTLSConfig()
		if err == nil || !strings.Contains(err.Error(), "defaultRootCA") {
			t.Fatalf("got err=%v", err)
		}
	})
	t.Run("invalid_PEM", func(t *testing.T) {
		g := NewGrpcSubscribe(WithServiceName("upstream"), EnableTls(), WithDefaultRootCA(func() []byte {
			return []byte("not-pem")
		}))
		_, err := g.buildClientTLSConfig()
		if err == nil || !strings.Contains(err.Error(), "root certificate") {
			t.Fatalf("got err=%v", err)
		}
	})
}

func TestGrpcSubscribe_buildClientTLSConfig_DefaultRootCA_OK(t *testing.T) {
	pemBytes := mustTestRootCAPEM(t)
	g := NewGrpcSubscribe(WithServiceName("upstream.example.com"), EnableTls(), WithDefaultRootCA(func() []byte {
		return pemBytes
	}))
	cfg, err := g.buildClientTLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || cfg.ServerName != "upstream.example.com" || cfg.RootCAs == nil {
		t.Fatalf("unexpected tls.Config: %+v", cfg)
	}
}

func TestGrpcSubscribe_buildClientTLSConfig_ConfigProvider_OK(t *testing.T) {
	pemBytes := mustTestRootCAPEM(t)
	payload, err := json.Marshal(map[string]string{"rootCA": string(pemBytes)})
	if err != nil {
		t.Fatal(err)
	}
	src := &staticConfigSource{kv: &config.KeyValue{Key: "tls", Format: "json", Value: payload}}

	g := NewGrpcSubscribe(
		WithServiceName("upstream.example.com"),
		EnableTls(),
		WithRootCAFileName("tls"),
		WithConfigProvider(func(name, group string) (config.Source, error) {
			if name != "tls" || group != "tls" {
				t.Fatalf("unexpected name/group: %q %q", name, group)
			}
			return src, nil
		}),
	)
	if g.caGroup != "" {
		t.Fatal("precondition: caGroup should be empty to exercise default-to-name branch")
	}
	cfg, err := g.buildClientTLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	if g.caGroup != "tls" {
		t.Fatalf("expected caGroup defaulted to caName, got %q", g.caGroup)
	}
	if cfg == nil || cfg.ServerName != "upstream.example.com" {
		t.Fatalf("unexpected tls.Config: %+v", cfg)
	}
}

func TestBuildGrpcSubscriptions_NilOrEmpty(t *testing.T) {
	t.Run("nil_cfg", func(t *testing.T) {
		conns, err := BuildGrpcSubscriptions(nil, nil, nil, nil)
		if err != nil || len(conns) != 0 {
			t.Fatalf("conns=%v err=%v", conns, err)
		}
	})
	t.Run("empty_grpc", func(t *testing.T) {
		conns, err := BuildGrpcSubscriptions(&conf.Subscriptions{}, nil, nil, nil)
		if err != nil || len(conns) != 0 {
			t.Fatalf("conns=%v err=%v", conns, err)
		}
	})
}

func TestBuildGrpcSubscriptions_SkipsEmptyServiceName(t *testing.T) {
	cfg := &conf.Subscriptions{
		Grpc: []*conf.GrpcSubscription{{Service: ""}},
	}
	conns, err := BuildGrpcSubscriptions(cfg, nil, nil, nil)
	if err != nil || len(conns) != 0 {
		t.Fatalf("conns=%v err=%v", conns, err)
	}
}

func TestBuildGrpcSubscriptionsLegacy_Delegates(t *testing.T) {
	a, err := BuildGrpcSubscriptionsLegacy(nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildGrpcSubscriptions(nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != len(b) {
		t.Fatalf("len mismatch: %d vs %d", len(a), len(b))
	}
}

// staticConfigSource is a minimal config.Source for tests (kratos v2).
type staticConfigSource struct {
	kv *config.KeyValue
}

func (s *staticConfigSource) Load() ([]*config.KeyValue, error) {
	return []*config.KeyValue{s.kv}, nil
}

func (s *staticConfigSource) Watch() (config.Watcher, error) {
	return &noopConfigWatcher{}, nil
}

type noopConfigWatcher struct{}

func (noopConfigWatcher) Next() ([]*config.KeyValue, error) {
	return nil, context.Canceled
}

func (noopConfigWatcher) Stop() error { return nil }

func mustTestRootCAPEM(t *testing.T) []byte {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{"subscribe-test-ca"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
