package boot

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	lynxapp "github.com/go-lynx/lynx"
)

// ---- HealthChecker tests ----

type testConfigSource struct {
	kv *config.KeyValue
}

type testConfigWatcher struct {
	stop     chan struct{}
	stopOnce sync.Once
}

func (s *testConfigSource) Load() ([]*config.KeyValue, error) {
	return []*config.KeyValue{s.kv}, nil
}

func (s *testConfigSource) Watch() (config.Watcher, error) {
	return &testConfigWatcher{stop: make(chan struct{})}, nil
}

func (w *testConfigWatcher) Next() ([]*config.KeyValue, error) {
	<-w.stop
	return nil, context.Canceled
}

func (w *testConfigWatcher) Stop() error {
	w.stopOnce.Do(func() {
		close(w.stop)
	})
	return nil
}

func newBootTestConfig(t *testing.T, name string) config.Config {
	t.Helper()

	cfg := config.New(
		config.WithSource(&testConfigSource{kv: &config.KeyValue{
			Key:    t.Name() + ".yaml",
			Format: "yaml",
			Value:  []byte("lynx:\n  application:\n    name: " + name + "\n    version: v0.0.1\n    close_banner: true\n"),
		}}),
	)
	if err := cfg.Load(); err != nil {
		t.Fatalf("failed to load boot test config: %v", err)
	}
	t.Cleanup(func() {
		_ = cfg.Close()
	})
	return cfg
}

func newTestHealthChecker(t *testing.T) *HealthChecker {
	t.Helper()
	hc := &HealthChecker{
		isHealthy:     true,
		lastCheck:     time.Now(),
		checkInterval: 10 * time.Millisecond,
		stopChan:      make(chan struct{}),
	}
	return hc
}

func TestHealthChecker_InitiallyHealthy(t *testing.T) {
	hc := newTestHealthChecker(t)
	if !hc.IsHealthy() {
		t.Error("expected health checker to be healthy initially")
	}
}

func TestHealthChecker_StopIdempotent(t *testing.T) {
	hc := newTestHealthChecker(t)
	// Stopping multiple times must not panic (sync.Once guard)
	hc.Stop()
	hc.Stop()
	hc.Stop()
}

func TestHealthChecker_RunAndStop(t *testing.T) {
	hc := &HealthChecker{
		isHealthy:     true,
		lastCheck:     time.Now(),
		checkInterval: 5 * time.Millisecond,
		stopChan:      make(chan struct{}),
	}
	done := make(chan struct{})
	go func() {
		hc.Run()
		close(done)
	}()
	time.Sleep(20 * time.Millisecond) // Let at least one tick fire
	hc.Stop()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Error("HealthChecker.Run did not return after Stop")
	}
}

func TestHealthChecker_IsHealthy_ConcurrentRead(t *testing.T) {
	hc := newTestHealthChecker(t)
	var wg sync.WaitGroup
	const n = 50
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_ = hc.IsHealthy()
		}()
	}
	wg.Wait()
}

// ---- formatStartupElapsed tests ----

func TestFormatStartupElapsed(t *testing.T) {
	tests := []struct {
		elapsed  time.Duration
		contains string
	}{
		{500 * time.Millisecond, "ms"},
		{5 * time.Second, "s"},
		{2 * time.Minute, "m"},
	}
	for _, tt := range tests {
		result := formatStartupElapsed(tt.elapsed)
		if result == "" {
			t.Errorf("formatStartupElapsed(%v) returned empty string", tt.elapsed)
		}
		if !strings.Contains(result, tt.contains) {
			t.Errorf("expected %q in formatStartupElapsed(%v) result %q", tt.contains, tt.elapsed, result)
		}
	}
}

// ---- Application construction tests ----

func TestNewApplication_NilWire(t *testing.T) {
	app := NewApplication(nil)
	if app != nil {
		t.Error("expected nil Application when wire is nil")
	}
}

func TestApplication_SetConfigPath(t *testing.T) {
	app := &Application{}
	result := app.SetConfigPath("/etc/lynx/config.yaml")
	if result != app {
		t.Error("expected SetConfigPath to return the same Application instance")
	}
	if app.configPath != "/etc/lynx/config.yaml" {
		t.Errorf("expected configPath '/etc/lynx/config.yaml', got %q", app.configPath)
	}
}

func TestApplication_SetConfigPath_Nil(t *testing.T) {
	var app *Application
	// Should not panic; returns nil
	result := app.SetConfigPath("/path")
	if result != nil {
		t.Error("expected nil return for nil Application")
	}
}

func TestApplication_SetPublishDefaultApp(t *testing.T) {
	app := &Application{publishDefaultApp: true}
	result := app.SetPublishDefaultApp(false)
	if result != app {
		t.Error("expected SetPublishDefaultApp to return the same Application instance")
	}
	if app.publishDefaultApp {
		t.Error("expected publishDefaultApp to be false after SetPublishDefaultApp(false)")
	}
}

func TestApplication_SetPublishDefaultApp_Nil(t *testing.T) {
	var app *Application
	result := app.SetPublishDefaultApp(true)
	if result != nil {
		t.Error("expected nil return for nil Application")
	}
}

func TestApplication_Run_NilInstance(t *testing.T) {
	var app *Application
	err := app.Run()
	if err == nil {
		t.Error("expected error when running nil application")
	}
}

func TestApplication_PublishAppIfConfigured(t *testing.T) {
	lynxapp.ClearDefaultApp()
	t.Cleanup(lynxapp.ClearDefaultApp)

	cfg := newBootTestConfig(t, "boot-publish")
	coreApp, err := lynxapp.NewStandaloneApp(cfg)
	if err != nil {
		t.Fatalf("expected standalone app: %v", err)
	}
	t.Cleanup(func() {
		_ = coreApp.Close()
	})

	shell := &Application{publishDefaultApp: true}
	shell.publishAppIfConfigured(coreApp)

	if got := lynxapp.Lynx(); got != coreApp {
		t.Fatalf("expected published default app, got %p want %p", got, coreApp)
	}
}

func TestApplication_PublishAppIfConfigured_Disabled(t *testing.T) {
	lynxapp.ClearDefaultApp()
	t.Cleanup(lynxapp.ClearDefaultApp)

	cfg := newBootTestConfig(t, "boot-no-publish")
	coreApp, err := lynxapp.NewStandaloneApp(cfg)
	if err != nil {
		t.Fatalf("expected standalone app: %v", err)
	}
	t.Cleanup(func() {
		_ = coreApp.Close()
	})

	shell := &Application{publishDefaultApp: false}
	shell.publishAppIfConfigured(coreApp)

	if got := lynxapp.Lynx(); got != nil {
		t.Fatalf("expected default app to remain nil when publication is disabled, got %p", got)
	}
}

// ---- initializeEnhancedFeatures tests ----

func TestApplication_InitializeEnhancedFeatures(t *testing.T) {
	app := &Application{}
	app.initializeEnhancedFeatures()

	if app.shutdownTimeout != DefaultShutdownTimeout {
		t.Errorf("expected default shutdown timeout %v, got %v", DefaultShutdownTimeout, app.shutdownTimeout)
	}
	if app.shutdownChan == nil {
		t.Error("expected shutdownChan to be initialized")
	}
	if app.healthChecker == nil {
		t.Error("expected healthChecker to be initialized")
	}
	if app.circuitBreaker == nil {
		t.Error("expected circuitBreaker to be initialized")
	}
	if !app.healthChecker.IsHealthy() {
		t.Error("expected health checker to start healthy")
	}
}

// ---- loadPluginsWithProtection with open circuit breaker ----

func TestApplication_LoadPluginsWithProtection_OpenBreaker(t *testing.T) {
	app := &Application{}
	app.circuitBreaker = lynxapp.NewCircuitBreaker(1, time.Minute)

	// Force the circuit breaker open
	app.circuitBreaker.RecordResult(errForTest("force open"))

	err := app.loadPluginsWithProtection()
	if err == nil {
		t.Error("expected error when circuit breaker is open")
	}
}

// ---- getters with nil conf ----

func TestApplication_GetName_NoConf(t *testing.T) {
	app := &Application{}
	name := app.GetName()
	if name != "lynx" {
		t.Errorf("expected default name 'lynx', got %q", name)
	}
}

func TestApplication_GetHost_NoConf(t *testing.T) {
	app := &Application{}
	host := app.GetHost()
	if host != "localhost" {
		t.Errorf("expected default host 'localhost', got %q", host)
	}
}

func TestApplication_GetVersion_NoConf(t *testing.T) {
	app := &Application{}
	version := app.GetVersion()
	if version != "unknown" {
		t.Errorf("expected default version 'unknown', got %q", version)
	}
}

// ---- isTestEnvironment ----

func TestIsTestEnvironment(t *testing.T) {
	// When running under `go test`, test.v flag is registered
	if !isTestEnvironment() {
		t.Error("expected isTestEnvironment() to return true in test context")
	}
}

// helper for creating errors in tests
type testError string

func (e testError) Error() string { return string(e) }
func errForTest(msg string) error { return testError(msg) }
