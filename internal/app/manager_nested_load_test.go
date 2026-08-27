package app

import (
	"errors"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/plugins"
)

// reentrantLoadPlugin mimics a control-plane plugin that loads further plugins
// from inside its own Start hook (polaris/nacos/etcd/apollo all do this).
type reentrantLoadPlugin struct {
	*plugins.BasePlugin
	manager   *DefaultPluginManager[plugins.Plugin]
	nested    config.Config
	nestedErr error
}

func (p *reentrantLoadPlugin) StartupTasks() error {
	p.nestedErr = p.manager.LoadPlugins(p.nested)
	return nil
}

func TestManager_LoadPlugins_ReentrantCallFromStartHookDoesNotDeadlock(t *testing.T) {
	manager, typedFactory := newFactoryBackedManager()
	nestedCfg := createManagerLoadConfig(t, "dependent:\n  enabled: true\n")
	typedFactory.RegisterPlugin("control-plane", "controlplane", func() plugins.Plugin {
		return &reentrantLoadPlugin{
			BasePlugin: plugins.NewBasePlugin("test.control-plane.v1", "control-plane", "cp", "v1.0.0", "test.control-plane", 0),
			manager:    manager,
			nested:     nestedCfg,
		}
	})
	typedFactory.RegisterPlugin("dependent", "dependent", func() plugins.Plugin {
		return newStartupControlledPlugin("dependent", nil)
	})

	cfg := createManagerLoadConfig(t, "controlplane:\n  enabled: true\n")

	done := make(chan error, 1)
	go func() { done <- manager.LoadPlugins(cfg) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected outer load to succeed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("outer LoadPlugins did not return: nested load deadlocked on operationMu")
	}

	names := ListPluginNames(manager)
	if len(names) != 2 {
		t.Fatalf("expected control-plane and dependent to be managed, got %v", names)
	}
	if manager.LoadInProgress() {
		t.Fatal("loadInProgress flag must be cleared after the load completes")
	}
	if len(manager.pendingLoads) != 0 {
		t.Fatalf("pending load queue must be drained, got %d", len(manager.pendingLoads))
	}
}

func TestManager_LoadPlugins_QueuedLoadFailurePropagatesToOuterCall(t *testing.T) {
	manager, typedFactory := newFactoryBackedManager()
	nestedCfg := createManagerLoadConfig(t, "broken:\n  enabled: true\n")
	typedFactory.RegisterPlugin("control-plane", "controlplane", func() plugins.Plugin {
		return &reentrantLoadPlugin{
			BasePlugin: plugins.NewBasePlugin("test.control-plane.v1", "control-plane", "cp", "v1.0.0", "test.control-plane", 0),
			manager:    manager,
			nested:     nestedCfg,
		}
	})
	startErr := errors.New("boom")
	typedFactory.RegisterPlugin("broken", "broken", func() plugins.Plugin {
		return newStartupControlledPlugin("broken", startErr)
	})

	cfg := createManagerLoadConfig(t, "controlplane:\n  enabled: true\n")
	err := manager.LoadPlugins(cfg)
	if err == nil || !errors.Is(err, startErr) {
		t.Fatalf("expected the queued load's failure to surface from the outer call, got %v", err)
	}
	if manager.LoadInProgress() {
		t.Fatal("loadInProgress flag must be cleared even when a queued load fails")
	}
}

func TestLynxApp_SetGlobalConfig_AllowedWhileBootstrapLoadInProgress(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	app := &LynxApp{pluginManager: manager, globalConf: createManagerLoadConfig(t, "a: 1\n")}

	// Simulate a sibling plugin already registered by the in-flight load.
	sibling := newConfigurableTestPlugin("sibling")
	if err := manager.registerPreparedPlugin(sibling); err != nil {
		t.Fatal(err)
	}
	if err := manager.registerManagedPlugin(sibling); err != nil {
		t.Fatal(err)
	}

	merged := createManagerLoadConfig(t, "b: 2\n")
	if err := app.SetGlobalConfig(merged); err == nil {
		t.Fatal("expected reload to be refused outside of a load operation")
	}

	manager.loadInProgress.Store(true)
	defer manager.loadInProgress.Store(false)
	if err := app.SetGlobalConfig(merged); err != nil {
		t.Fatalf("expected config swap to be allowed during bootstrap load: %v", err)
	}
	if app.globalConf != merged {
		t.Fatal("global config was not replaced")
	}
}

func TestManager_UnloadPluginsByName_SuspendedPluginIsStoppedNotRecordedAsFailure(t *testing.T) {
	manager, typedFactory := newFactoryBackedManager()
	typedFactory.RegisterPlugin("suspendable", "suspendable", func() plugins.Plugin {
		return newStartupControlledPlugin("suspendable", nil)
	})
	cfg := createManagerLoadConfig(t, "suspendable:\n  enabled: true\n")
	if err := manager.LoadPlugins(cfg); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	p := manager.GetPlugin("suspendable")
	sp, ok := p.(interface{ Suspend() error })
	if !ok {
		t.Fatal("test plugin does not support Suspend")
	}
	if err := sp.Suspend(); err != nil {
		t.Fatalf("suspend failed: %v", err)
	}
	if got := p.Status(p); got != plugins.StatusSuspended {
		t.Fatalf("expected suspended status, got %v", got)
	}

	manager.UnloadPluginsByName([]string{"suspendable"})
	if failures := manager.GetUnloadFailures(); len(failures) != 0 {
		t.Fatalf("suspended plugin unload must not be recorded as a failure: %#v", failures)
	}
	if got := ListPluginNames(manager); len(got) != 0 {
		t.Fatalf("expected plugin to be unregistered, got %v", got)
	}
}

func TestTopologicalSort_EnforcesDeclaredVersionConstraints(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	dep := newConfigurableTestPlugin("dep-v1") // version v1.0.0
	consumer := newConfigurableTestPlugin("consumer")
	consumer.AddDependency(plugins.Dependency{
		ID:                dep.ID(),
		Type:              plugins.DependencyTypeRequired,
		Required:          true,
		VersionConstraint: &plugins.VersionConstraint{MinVersion: "2.0.0"},
	})

	if _, err := manager.TopologicalSort([]plugins.Plugin{dep, consumer}); err == nil {
		t.Fatal("expected version constraint violation to fail the sort")
	}

	ok := newConfigurableTestPlugin("consumer-ok")
	ok.AddDependency(plugins.Dependency{
		ID:                dep.ID(),
		Type:              plugins.DependencyTypeRequired,
		Required:          true,
		VersionConstraint: &plugins.VersionConstraint{MinVersion: "1.0.0"},
	})
	if _, err := manager.TopologicalSort([]plugins.Plugin{dep, ok}); err != nil {
		t.Fatalf("satisfied constraint must not fail: %v", err)
	}
}

func TestPreparePlug_SkipsPrefixWithEnabledFalse(t *testing.T) {
	manager, typedFactory := newFactoryBackedManager()
	typedFactory.RegisterPlugin("optional", "optional", func() plugins.Plugin {
		return newStartupControlledPlugin("optional", nil)
	})

	disabled := createManagerLoadConfig(t, "optional:\n  enabled: false\n  addr: example\n")
	prepared, err := manager.PreparePlug(disabled)
	if err != nil {
		t.Fatalf("PreparePlug: %v", err)
	}
	if len(prepared) != 0 {
		t.Fatalf("enabled: false must skip the plugin, prepared %d", len(prepared))
	}

	enabled := createManagerLoadConfig(t, "optional:\n  addr: example\n")
	prepared, err = manager.PreparePlug(enabled)
	if err != nil {
		t.Fatalf("PreparePlug: %v", err)
	}
	if len(prepared) != 1 {
		t.Fatalf("missing enabled key must keep loading the plugin, prepared %d", len(prepared))
	}
}
