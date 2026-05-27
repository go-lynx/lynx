package app

import (
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/plugins"
)

func TestLifecyclePolicy_ProductionRequiresTrueContextLifecycle(t *testing.T) {
	t.Setenv("LYNX_ENV", "production")
	manager := NewPluginManager[plugins.Plugin]()

	err := manager.enforceLifecyclePolicy([]plugins.Plugin{&FastPlugin{}})
	if err == nil || !strings.Contains(err.Error(), "not production-safe") {
		t.Fatalf("enforceLifecyclePolicy error = %v, want production-safe rejection", err)
	}
}

func TestLifecyclePolicy_ProductionConfigWithoutOverrideStillRequiresContextLifecycle(t *testing.T) {
	t.Setenv("LYNX_ENV", "production")
	manager := NewPluginManager[plugins.Plugin]()
	manager.SetConfig(createTestConfig(t))

	err := manager.enforceLifecyclePolicy([]plugins.Plugin{&FastPlugin{}})
	if err == nil || !strings.Contains(err.Error(), "not production-safe") {
		t.Fatalf("enforceLifecyclePolicy error = %v, want production-safe rejection", err)
	}
}

func TestLifecyclePolicy_ContextAwarePluginPassesProductionPolicy(t *testing.T) {
	t.Setenv("LYNX_ENV", "production")
	manager := NewPluginManager[plugins.Plugin]()

	err := manager.enforceLifecyclePolicy([]plugins.Plugin{&ContextAwareSlowPlugin{}})
	if err != nil {
		t.Fatalf("enforceLifecyclePolicy returned error for context-aware plugin: %v", err)
	}
}

func TestLifecyclePolicy_ConfigCanDisableProductionRequirement(t *testing.T) {
	t.Setenv("LYNX_ENV", "production")
	cfg := config.New(
		config.WithSource(&staticSource{kv: &config.KeyValue{
			Key:    t.Name() + "-policy.yaml",
			Format: "yaml",
			Value:  []byte("lynx:\n  plugins:\n    require_context_aware_lifecycle: false\n"),
		}}),
	)
	if err := cfg.Load(); err != nil {
		t.Fatalf("cfg.Load: %v", err)
	}
	t.Cleanup(func() { _ = cfg.Close() })

	manager := NewPluginManager[plugins.Plugin]()
	manager.SetConfig(cfg)

	err := manager.enforceLifecyclePolicy([]plugins.Plugin{&FastPlugin{}})
	if err != nil {
		t.Fatalf("enforceLifecyclePolicy returned error despite explicit override: %v", err)
	}
}
