package app

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/log"
	"github.com/go-lynx/lynx/pkg/security"
	"github.com/go-lynx/lynx/plugins"
)

const requireContextAwareLifecycleKey = "lynx.plugins.require_context_aware_lifecycle"

// lifecyclePolicyMode describes how non-cancellable plugins are handled.
type lifecyclePolicyMode int

const (
	lifecyclePolicyAllow  lifecyclePolicyMode = iota // no check
	lifecyclePolicyWarn                              // production default: log, keep loading
	lifecyclePolicyReject                            // explicit lynx.plugins.require_context_aware_lifecycle=true
)

// lifecyclePolicy resolves the policy: an explicit config value wins (true →
// reject, false → allow); otherwise production only warns. Rejecting by default
// in production turned out to block widely used plugins (mysql/redis/kafka…)
// that had not migrated to context-aware hooks yet, so the hard failure is
// opt-in until every first-party plugin is cancellable.
func (m *DefaultPluginManager[T]) lifecyclePolicy() lifecyclePolicyMode {
	if conf := m.getConfigSnapshot(); conf != nil {
		var required bool
		if err := conf.Value(requireContextAwareLifecycleKey).Scan(&required); err == nil {
			if required {
				return lifecyclePolicyReject
			}
			return lifecyclePolicyAllow
		}
	}
	if security.IsProduction() {
		return lifecyclePolicyWarn
	}
	return lifecyclePolicyAllow
}

func (m *DefaultPluginManager[T]) enforceLifecyclePolicy(plugs []plugins.Plugin) error {
	mode := m.lifecyclePolicy()
	if mode == lifecyclePolicyAllow {
		return nil
	}
	for _, p := range plugs {
		if p == nil || plugins.HasTrueContextLifecycle(p) {
			continue
		}
		msg := fmt.Sprintf(
			"plugin %s (%s) does not have a genuinely cancellable lifecycle — implement a context-aware step hook (e.g. StartupTasksContext), or declare PluginProtocol().ContextLifecycle with LifecycleWithContext and IsContextAware()=true",
			p.Name(), p.ID())
		if mode == lifecyclePolicyReject {
			return fmt.Errorf("%s=true: %s", requireContextAwareLifecycleKey, msg)
		}
		log.Warnf("production lifecycle policy: %s (set %s=true to make this an error)", msg, requireContextAwareLifecycleKey)
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
