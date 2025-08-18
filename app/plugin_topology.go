// Package app: plugin dependency topology utilities.
package app

import (
	"github.com/go-lynx/lynx/plugins"
)

// PluginWithLevel represents a plugin with its dependency level.
type PluginWithLevel struct {
	plugins.Plugin
	level int
}

// TopologicalSort sorts a subset of plugins based on required dependencies
// and calculates an integer level for each plugin.
func (m *DefaultPluginManager[T]) TopologicalSort(plugs []plugins.Plugin) ([]PluginWithLevel, error) {
	if len(plugs) == 0 {
		return nil, nil
	}

	id2plugin := make(map[string]plugins.Plugin, len(plugs))
	for _, p := range plugs {
		if p == nil {
			continue
		}
		id2plugin[p.ID()] = p
	}

	dg := plugins.NewDependencyGraph()
	for _, p := range id2plugin {
		dg.AddPlugin(p)
	}

	requiredDeps := make(map[string][]string, len(id2plugin))
	for _, p := range id2plugin {
		deps := p.GetDependencies()
		if len(deps) == 0 {
			continue
		}
		for _, dep := range deps {
			if dep.Type != plugins.DependencyTypeRequired {
				continue
			}
			if _, ok := id2plugin[dep.ID]; !ok {
				continue
			}
			d := dep
			if err := dg.AddDependency(p.ID(), &d); err != nil {
				return nil, err
			}
			requiredDeps[p.ID()] = append(requiredDeps[p.ID()], dep.ID)
		}
	}

	orderedIDs, err := dg.ResolveDependencies()
	if err != nil {
		return nil, err
	}

	memo := make(map[string]int, len(id2plugin))
	visiting := make(map[string]bool, len(id2plugin))
	var dfs func(string) int
	dfs = func(id string) int {
		if lv, ok := memo[id]; ok {
			return lv
		}
		if visiting[id] {
			return 0
		}
		visiting[id] = true
		best := 0
		for _, depID := range requiredDeps[id] {
			lv := dfs(depID)
			if lv+1 > best {
				best = lv + 1
			}
		}
		visiting[id] = false
		memo[id] = best
		return best
	}

	result := make([]PluginWithLevel, 0, len(id2plugin))
	for _, id := range orderedIDs {
		p, ok := id2plugin[id]
		if !ok || p == nil {
			continue
		}
		result = append(result, PluginWithLevel{Plugin: p, level: dfs(id)})
	}
	return result, nil
}
