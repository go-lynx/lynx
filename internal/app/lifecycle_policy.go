package app

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/pkg/security"
	"github.com/go-lynx/lynx/plugins"
)

const requireContextAwareLifecycleKey = "lynx.plugins.require_context_aware_lifecycle"

func (m *DefaultPluginManager[T]) requireContextAwareLifecycle() bool {
	if conf := m.getConfigSnapshot(); conf != nil {
		var required bool
		if err := conf.Value(requireContextAwareLifecycleKey).Scan(&required); err == nil {
			return required
		}
	}
	return security.IsProduction()
}

func (m *DefaultPluginManager[T]) enforceLifecyclePolicy(plugs []plugins.Plugin) error {
	if !m.requireContextAwareLifecycle() {
		return nil
	}
	for _, p := range plugs {
		if p == nil {
			continue
		}
		if !plugins.HasTrueContextLifecycle(p) {
			return fmt.Errorf(
				"plugin %s (%s) is not production-safe: %s=true requires a genuinely cancellable lifecycle — implement a context-aware step hook (e.g. StartupTasksContext), or declare PluginProtocol().ContextLifecycle with LifecycleWithContext and IsContextAware()=true",
				p.Name(),
				p.ID(),
				requireContextAwareLifecycleKey,
			)
		}
	}
	return nil
}

func contextAwareLifecycleRequired(conf config.Config) bool {
	if conf != nil {
		var required bool
		if err := conf.Value(requireContextAwareLifecycleKey).Scan(&required); err == nil {
			return required
		}
	}
	return security.IsProduction()
}
