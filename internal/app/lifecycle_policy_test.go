package app

import (
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/plugins"
)

func TestLifecyclePolicy_ProductionOnlyWarnsByDefault(t *testing.T) {
	t.Setenv("LYNX_ENV", "production")
	manager := NewPluginManager[plugins.Plugin]()

	if err := manager.enforceLifecyclePolicy([]plugins.Plugin{&FastPlugin{}}); err != nil {
		t.Fatalf("production default must warn, not reject: %v", err)
	}
	if got := manager.lifecyclePolicy(); got != lifecyclePolicyWarn {
		t.Fatalf("lifecyclePolicy() = %v, want warn", got)
	}
}

func TestLifecyclePolicy_ExplicitRequirementRejectsNonCancellablePlugin(t *testing.T) {
	cfg := config.New(
		config.WithSource(&staticSource{kv: &config.KeyValue{
			Key:    t.Name() + "-policy.yaml",
			Format: "yaml",
			Value:  []byte("lynx:\n  plugins:\n    require_context_aware_lifecycle: true\n"),
		}}),
	)
	if err := cfg.Load(); err != nil {
		t.Fatalf("cfg.Load: %v", err)
	}
	t.Cleanup(func() { _ = cfg.Close() })

	manager := NewPluginManager[plugins.Plugin]()
	manager.SetConfig(cfg)

	err := manager.enforceLifecyclePolicy([]plugins.Plugin{&FastPlugin{}})
	if err == nil || !strings.Contains(err.Error(), requireContextAwareLifecycleKey) {
		t.Fatalf("enforceLifecyclePolicy error = %v, want explicit rejection", err)
	}
	if err := manager.enforceLifecyclePolicy([]plugins.Plugin{&ContextAwareSlowPlugin{}}); err != nil {
		t.Fatalf("context-aware plugin must pass even under explicit requirement: %v", err)
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
