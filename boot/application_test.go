package boot

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2"
	lynxapp "github.com/go-lynx/lynx"
)

func TestNewApplication_DefaultsToPublishingDefaultApp(t *testing.T) {
	app := NewApplication(func() (*kratos.App, error) { return nil, nil })
	if app == nil {
		t.Fatal("expected application to be created")
	}
	if !app.publishDefaultApp {
		t.Fatal("expected new boot application to publish default app by default")
	}
}

func TestApplication_SettersSupportInstanceScopedOverrides(t *testing.T) {
	app := NewApplication(func() (*kratos.App, error) { return nil, nil })
	if app == nil {
		t.Fatal("expected application to be created")
	}

	got := app.SetConfigPath("/tmp/bootstrap.yaml").SetPublishDefaultApp(false)
	if got != app {
		t.Fatal("expected setters to support fluent chaining")
	}
	if app.configPath != "/tmp/bootstrap.yaml" {
		t.Fatalf("expected instance config path to be set, got %q", app.configPath)
	}
	if app.publishDefaultApp {
		t.Fatal("expected publishDefaultApp to be disabled")
	}
}

func TestApplication_PublishAppIfConfigured_RespectsFlag(t *testing.T) {
	lynxapp.ClearDefaultApp()
	t.Cleanup(lynxapp.ClearDefaultApp)

	explicitApp := &lynxapp.LynxApp{}
	app := NewApplication(func() (*kratos.App, error) { return nil, nil }).SetPublishDefaultApp(false)
	app.publishAppIfConfigured(explicitApp)
	if got := lynxapp.Lynx(); got != nil {
		t.Fatalf("expected no default app to be published when disabled, got %p", got)
	}

	app.SetPublishDefaultApp(true)
	app.publishAppIfConfigured(explicitApp)
	if got := lynxapp.Lynx(); got != explicitApp {
		t.Fatalf("expected explicit app to be published, got %p want %p", got, explicitApp)
	}
}

func TestFormatStartupElapsed(t *testing.T) {
	cases := []struct {
		name string
		in   time.Duration
		want string
	}{
		{name: "milliseconds", in: 850 * time.Millisecond, want: "850 ms"},
		{name: "seconds", in: 2500 * time.Millisecond, want: "2.50 s"},
		{name: "minutes", in: 125 * time.Second, want: "2.08 m"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatStartupElapsed(tc.in); got != tc.want {
				t.Fatalf("unexpected elapsed display: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestApplication_InitializeRuntimeShell_RequiresPluginManager(t *testing.T) {
	lynxapp.ClearDefaultApp()
	t.Cleanup(lynxapp.ClearDefaultApp)

	app := NewApplication(func() (*kratos.App, error) { return nil, nil }).SetPublishDefaultApp(false)
	app.conf = nil

	err := app.initializeRuntimeShell(&lynxapp.LynxApp{})
	if err == nil {
		t.Fatal("expected initializeRuntimeShell to reject app without plugin manager")
	}
}

func TestApplication_ProtectShutdownStep_RecoversPanic(t *testing.T) {
	app := &Application{}

	called := false
	app.protectShutdownStep("test shutdown step", func() {
		called = true
		panic("boom")
	})

	if !called {
		t.Fatal("expected protected shutdown step to run")
	}
}

func TestApplication_StopKratosAppWithTimeout(t *testing.T) {
	app := &Application{shutdownTimeout: 20 * time.Millisecond}
	kratosApp := kratos.New(
		kratos.BeforeStop(func(context.Context) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		}),
	)

	err := app.stopKratosAppWithTimeout(kratosApp)
	if err == nil {
		t.Fatal("expected stopKratosAppWithTimeout to time out")
	}
	if !strings.Contains(err.Error(), "shutdown timeout exceeded") {
		t.Fatalf("expected shutdown timeout error, got %v", err)
	}
}
