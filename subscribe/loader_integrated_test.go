package subscribe

// 这些用例覆盖 loader_integrated.go 中的占位 API：实现真实逻辑后请同步更新或删除相关断言。

import (
	"strings"
	"testing"

	"github.com/go-lynx/lynx/conf"
)

func TestBuildGrpcSubscriptionsWithPlugin_DelegatesToBuild(t *testing.T) {
	got, err := BuildGrpcSubscriptionsWithPlugin(nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want, err := BuildGrpcSubscriptions(nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("len got=%d want=%d", len(got), len(want))
	}
}

func TestLoaderIntegrated_StubsReturnErrors(t *testing.T) {
	stubs := []struct {
		name string
		fn   func() error
	}{
		{"GetGrpcConnection", func() error {
			_, err := GetGrpcConnection("svc", nil)
			return err
		}},
		{"CloseGrpcConnection", func() error { return CloseGrpcConnection("svc", nil) }},
		{"GetGrpcConnectionStatus", func() error {
			_, err := GetGrpcConnectionStatus(nil)
			return err
		}},
		{"GetGrpcConnectionCount", func() error {
			_, err := GetGrpcConnectionCount(nil)
			return err
		}},
		{"HealthCheckGrpcConnections", func() error { return HealthCheckGrpcConnections(nil) }},
		{"GetGrpcMetrics", func() error {
			_, err := GetGrpcMetrics(nil)
			return err
		}},
		{"InitializeGrpcClientIntegration", func() error { return InitializeGrpcClientIntegration(nil) }},
		{"CreateGrpcConnectionWithConfig", func() error {
			_, err := CreateGrpcConnectionWithConfig(nil, nil)
			return err
		}},
		{"GetGrpcClientPlugin", func() error {
			_, err := GetGrpcClientPlugin(nil)
			return err
		}},
	}
	for _, tc := range stubs {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fn()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "needs to be implemented") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestCreateGrpcConnectionWithConfig_NonNilConfigStillStub(t *testing.T) {
	cfg := &conf.Subscriptions{}
	_, err := CreateGrpcConnectionWithConfig(cfg, nil)
	if err == nil || !strings.Contains(err.Error(), "needs to be implemented") {
		t.Fatalf("got err=%v", err)
	}
}
