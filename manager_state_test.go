package lynx

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/pkg/factory"
	"github.com/go-lynx/lynx/plugins"
)

func TestManager_PreparedPluginsAreNotRuntimeVisibleUntilManaged(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()

	prepared := newConfigurableTestPlugin("prepared-only")
	if err := manager.registerPreparedPlugin(prepared); err != nil {
		t.Fatalf("failed to register prepared plugin: %v", err)
	}

	if got := Plugins(manager); len(got) != 0 {
		t.Fatalf("expected no managed plugins after prepare, got %d", len(got))
	}
	if got := ListPluginNames(manager); len(got) != 0 {
		t.Fatalf("expected no managed plugin names after prepare, got %d", len(got))
	}
	if got := manager.GetPlugin(prepared.Name()); got == nil {
		t.Fatal("expected prepared plugin to remain discoverable from inventory registry")
	}

	if err := manager.registerManagedPlugin(prepared); err != nil {
		t.Fatalf("failed to register managed plugin: %v", err)
	}

	managed := Plugins(manager)
	if len(managed) != 1 || managed[0].Name() != prepared.Name() {
		t.Fatalf("unexpected managed plugins after activation: %#v", managed)
	}
	names := ListPluginNames(manager)
	if len(names) != 1 || names[0] != prepared.Name() {
		t.Fatalf("unexpected managed plugin names after activation: %#v", names)
	}
}

func TestManager_PrepareSkipsAlreadyManagedPlugin(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()

	prepared := newConfigurableTestPlugin("managed-skip")
	if err := manager.registerPreparedPlugin(prepared); err != nil {
		t.Fatalf("failed to register prepared plugin: %v", err)
	}
	if err := manager.registerManagedPlugin(prepared); err != nil {
		t.Fatalf("failed to register managed plugin: %v", err)
	}

	got, err := manager.preparePlugin(prepared.Name())
	if err != nil {
		t.Fatalf("expected already managed plugin to be skipped without error: %v", err)
	}
	if got != nil {
		t.Fatal("expected no prepared plugin result for already managed plugin")
	}

	if manager.pluginList != nil && len(manager.pluginList) != 0 {
		t.Fatalf("expected prepared staging to be cleared after managed registration, got %d staged plugins", len(manager.pluginList))
	}
}

type startupControlledPlugin struct {
	*plugins.BasePlugin
	startErr error
	started  *atomic.Int32
}

type stopControlledPlugin struct {
	*plugins.BasePlugin
	stopErr error
}

func newStartupControlledPlugin(name string, startErr error) *startupControlledPlugin {
	return newCountingStartupControlledPlugin(name, startErr, nil)
}

func newCountingStartupControlledPlugin(name string, startErr error, started *atomic.Int32) *startupControlledPlugin {
	return &startupControlledPlugin{
		BasePlugin: plugins.NewBasePlugin("test."+name+".v1", name, "startup controlled plugin", "v1.0.0", "test."+name, 0),
		startErr:   startErr,
		started:    started,
	}
}

func newStopControlledPlugin(name string, stopErr error) *stopControlledPlugin {
	return &stopControlledPlugin{
		BasePlugin: plugins.NewBasePlugin("test."+name+".v1", name, "stop controlled plugin", "v1.0.0", "test."+name, 0),
		stopErr:    stopErr,
	}
}

func (p *startupControlledPlugin) StartupTasks() error {
	if p.started != nil {
		p.started.Add(1)
	}
	return p.startErr
}

func (p *stopControlledPlugin) Stop(plugin plugins.Plugin) error {
	return p.stopErr
}

func createManagerLoadConfig(t *testing.T, yamlBody string) config.Config {
	t.Helper()

	cfg := config.New(
		config.WithSource(&staticSource{kv: &config.KeyValue{
			Key:    t.Name() + "-plugins.yaml",
			Format: "yaml",
			Value:  []byte(yamlBody),
		}}),
	)
	if err := cfg.Load(); err != nil {
		t.Fatalf("failed to load manager config: %v", err)
	}
	t.Cleanup(func() {
		_ = cfg.Close()
	})
	return cfg
}

func newFactoryBackedManager() (*DefaultPluginManager[plugins.Plugin], *factory.TypedFactory) {
	manager := NewPluginManager[plugins.Plugin]()
	typedFactory := factory.NewTypedFactory()
	manager.factory = typedFactory
	return manager, typedFactory
}

func TestManager_LoadPlugins_IsNoOpWhenAlreadyManaged(t *testing.T) {
	manager, typedFactory := newFactoryBackedManager()
	typedFactory.RegisterPlugin("repeat-load", "repeat", func() plugins.Plugin {
		return newStartupControlledPlugin("repeat-load", nil)
	})

	cfg := createManagerLoadConfig(t, "repeat:\n  enabled: true\n")

	if err := manager.LoadPlugins(cfg); err != nil {
		t.Fatalf("expected first load to succeed: %v", err)
	}
	if got := Plugins(manager); len(got) != 1 || got[0].Name() != "repeat-load" {
		t.Fatalf("unexpected managed plugins after first load: %#v", got)
	}
	if manager.pluginList != nil && len(manager.pluginList) != 0 {
		t.Fatalf("expected staging to be empty after first load, got %d staged plugins", len(manager.pluginList))
	}

	if err := manager.LoadPlugins(cfg); err != nil {
		t.Fatalf("expected second load to become a no-op: %v", err)
	}
	if got := Plugins(manager); len(got) != 1 || got[0].Name() != "repeat-load" {
		t.Fatalf("unexpected managed plugins after second load: %#v", got)
	}
	if len(manager.managedPluginList) != 1 {
		t.Fatalf("expected exactly one managed plugin after repeated load, got %d", len(manager.managedPluginList))
	}
}

func TestManager_LoadPluginsByName_CleansUnusedPreparedStaging(t *testing.T) {
	manager, typedFactory := newFactoryBackedManager()
	typedFactory.RegisterPlugin("subset-alpha", "subset", func() plugins.Plugin {
		return newStartupControlledPlugin("subset-alpha", nil)
	})
	typedFactory.RegisterPlugin("subset-beta", "subset", func() plugins.Plugin {
		return newStartupControlledPlugin("subset-beta", nil)
	})

	cfg := createManagerLoadConfig(t, "subset:\n  enabled: true\n")

	if err := manager.LoadPluginsByName(cfg, []string{"subset-alpha"}); err != nil {
		t.Fatalf("expected subset load to succeed: %v", err)
	}

	if got := Plugins(manager); len(got) != 1 || got[0].Name() != "subset-alpha" {
		t.Fatalf("unexpected managed plugins after subset load: %#v", got)
	}
	if got := manager.GetPlugin("subset-beta"); got != nil {
		t.Fatalf("expected non-target plugin to be absent from inventory after subset load cleanup, got %s", got.Name())
	}
	if manager.pluginList != nil && len(manager.pluginList) != 0 {
		t.Fatalf("expected no staged plugins after subset load cleanup, got %d", len(manager.pluginList))
	}
}

func TestManager_LoadPlugins_RetryAfterStartupFailure(t *testing.T) {
	manager, typedFactory := newFactoryBackedManager()
	attempts := 0
	typedFactory.RegisterPlugin("retryable", "retryable", func() plugins.Plugin {
		attempts++
		if attempts == 1 {
			return newStartupControlledPlugin("retryable", fmt.Errorf("first startup failure"))
		}
		return newStartupControlledPlugin("retryable", nil)
	})

	cfg := createManagerLoadConfig(t, "retryable:\n  enabled: true\n")

	if err := manager.LoadPlugins(cfg); err == nil {
		t.Fatal("expected first load to fail")
	}
	if got := Plugins(manager); len(got) != 0 {
		t.Fatalf("expected no managed plugins after failed load, got %#v", got)
	}
	if got := manager.GetPlugin("retryable"); got != nil {
		t.Fatalf("expected failed plugin to be removed from registries after rollback, got %s", got.Name())
	}
	if manager.pluginList != nil && len(manager.pluginList) != 0 {
		t.Fatalf("expected failed load to leave no staged plugins, got %d", len(manager.pluginList))
	}

	if err := manager.LoadPlugins(cfg); err != nil {
		t.Fatalf("expected second load retry to succeed: %v", err)
	}
	if got := Plugins(manager); len(got) != 1 || got[0].Name() != "retryable" {
		t.Fatalf("unexpected managed plugins after retry: %#v", got)
	}
}

func TestManager_UnloadPluginsByName_RemovesOnlyTargetedManagedPlugin(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()

	alpha := newStartupControlledPlugin("subset-unload-alpha", nil)
	beta := newStartupControlledPlugin("subset-unload-beta", nil)
	alpha.SetStatus(plugins.StatusActive)
	beta.SetStatus(plugins.StatusActive)

	if err := manager.registerManagedPlugin(alpha); err != nil {
		t.Fatalf("failed to register alpha plugin: %v", err)
	}
	if err := manager.registerManagedPlugin(beta); err != nil {
		t.Fatalf("failed to register beta plugin: %v", err)
	}

	manager.UnloadPluginsByName([]string{alpha.Name()})

	if got := manager.GetPlugin(alpha.Name()); got != nil {
		t.Fatalf("expected targeted plugin to be removed after subset unload, got %s", got.Name())
	}
	if got := manager.GetPlugin(beta.Name()); got == nil || got.Name() != beta.Name() {
		t.Fatalf("expected non-target plugin to remain managed after subset unload, got %#v", got)
	}
}

func TestManager_StopPlugin_KeepsManagedRegistrationOnStopFailure(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()

	plugin := newStopControlledPlugin("stop-failure", fmt.Errorf("stop failed"))
	plugin.SetStatus(plugins.StatusActive)

	if err := manager.registerManagedPlugin(plugin); err != nil {
		t.Fatalf("failed to register plugin: %v", err)
	}

	err := manager.StopPlugin(plugin.Name())
	if err == nil {
		t.Fatal("expected stop failure to be returned")
	}
	if got := manager.GetPlugin(plugin.Name()); got == nil {
		t.Fatal("expected plugin to remain registered after stop failure")
	}
}

func TestManager_LoadPlugins_ConcurrentCallsSerializeStartup(t *testing.T) {
	manager, typedFactory := newFactoryBackedManager()
	var startCount atomic.Int32
	typedFactory.RegisterPlugin("concurrent-load", "concurrent", func() plugins.Plugin {
		return newCountingStartupControlledPlugin("concurrent-load", nil, &startCount)
	})

	cfg := createManagerLoadConfig(t, "concurrent:\n  enabled: true\n")

	var wg sync.WaitGroup
	errCh := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- manager.LoadPlugins(cfg)
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("expected concurrent load calls to succeed, got %v", err)
		}
	}

	if got := Plugins(manager); len(got) != 1 || got[0].Name() != "concurrent-load" {
		t.Fatalf("unexpected managed plugins after concurrent load: %#v", got)
	}
	if len(manager.managedPluginList) != 1 {
		t.Fatalf("expected exactly one managed plugin after concurrent load, got %d", len(manager.managedPluginList))
	}
	if got := startCount.Load(); got != 1 {
		t.Fatalf("expected plugin startup tasks to run exactly once under concurrent load, got %d", got)
	}
}

func TestNilManager_OperationsRemainSafe(t *testing.T) {
	var manager *DefaultPluginManager[plugins.Plugin]

	if err := manager.LoadPlugins(nil); err == nil {
		t.Fatal("expected nil manager load to return an error")
	}
	if err := manager.LoadPluginsByName(nil, []string{"missing"}); err == nil {
		t.Fatal("expected nil manager subset load to return an error")
	}
	if err := manager.StopPlugin("missing"); err == nil {
		t.Fatal("expected nil manager stop to return an error")
	}

	manager.UnloadPlugins()
	manager.UnloadPluginsByName([]string{"missing"})
}
