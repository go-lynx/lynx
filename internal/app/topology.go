// Plugin dependency resolution: topological sort plus per-plugin level
// calculation so plugins at the same level can be started in parallel.
package app

import (
	"fmt"
	"strings"

	"github.com/go-lynx/lynx/plugins"
)

// PluginWithLevel represents a plugin with its dependency level.
// The level indicates how deep in the dependency chain this plugin is,
// with 0 being plugins with no dependencies.
type PluginWithLevel struct {
	plugins.Plugin
	level int
}

// TopologicalSort orders plugins by their required dependencies and assigns each
// a level equal to the maximum depth of its dependency chain (level 0 = no deps).
// Returns an error on circular or missing required dependencies.
//
// Dependency timing: GetDependencies() is called before any plugin is initialized.
// Required dependencies that affect load order must therefore be declared in the
// plugin constructor (or before this sort runs), not only in InitializeResources.
// See plugins package README "Dependency declaration timing".
func (m *DefaultPluginManager[T]) TopologicalSort(plugs []plugins.Plugin) ([]PluginWithLevel, error) {
	if len(plugs) == 0 {
		return nil, nil
	}

	id2plugin := make(map[string]plugins.Plugin, len(plugs))
	name2plugin := make(map[string]plugins.Plugin, len(plugs))
	for _, p := range plugs {
		if p == nil {
			continue
		}
		id := p.ID()
		if existing, exists := id2plugin[id]; exists {
			return nil, fmt.Errorf("duplicate plugin ID detected: %s (plugins: %s and %s)",
				id, existing.Name(), p.Name())
		}
		id2plugin[id] = p
		if name := p.Name(); name != "" {
			name2plugin[name] = p
		}
	}

	if len(id2plugin) == 0 {
		return nil, fmt.Errorf("no valid plugins to sort (all plugins are nil)")
	}

	dg := plugins.NewDependencyGraph()
	for _, p := range id2plugin {
		dg.AddPlugin(p)
	}

	requiredDeps := make(map[string][]string, len(id2plugin))
	missingDeps := make(map[string][]string) // pluginID -> []missingDepID

	for _, p := range id2plugin {
		deps := p.GetDependencies()
		if len(deps) == 0 {
			continue
		}

		for _, dep := range deps {
			if dep.Type != plugins.DependencyTypeRequired {
				continue
			}

			resolvedID, ok := resolveRequiredDependencyID(id2plugin, name2plugin, dep)
			if !ok {
				missingDeps[p.ID()] = append(missingDeps[p.ID()], dependencyDisplayName(dep))
				continue
			}

			d := dep
			d.ID = resolvedID
			if err := dg.AddDependency(p.ID(), &d); err != nil {
				return nil, fmt.Errorf("failed to add dependency %s -> %s: %w", p.ID(), resolvedID, err)
			}
			requiredDeps[p.ID()] = append(requiredDeps[p.ID()], resolvedID)
		}
	}

	if len(missingDeps) > 0 {
		var missingMsgs []string
		for pluginID, missing := range missingDeps {
			missingMsgs = append(missingMsgs, fmt.Sprintf("plugin %s requires missing dependencies: [%s]",
				pluginID, strings.Join(missing, ", ")))
		}
		return nil, fmt.Errorf("missing required dependencies: %s", strings.Join(missingMsgs, "; "))
	}

	orderedIDs, err := dg.ResolveDependencies()
	if err != nil {
		return nil, fmt.Errorf("dependency resolution failed: %w", err)
	}

	// Completeness guard: every input plugin must appear exactly once in the result.
	if len(orderedIDs) != len(id2plugin) {
		orderedSet := make(map[string]bool, len(orderedIDs))
		var duplicates []string
		for _, id := range orderedIDs {
			if orderedSet[id] {
				duplicates = append(duplicates, id)
			}
			orderedSet[id] = true
		}

		var missing []string
		for id := range id2plugin {
			if !orderedSet[id] {
				missing = append(missing, id)
			}
		}

		errMsg := fmt.Sprintf("topological sort incomplete: %d plugins in result, %d expected",
			len(orderedIDs), len(id2plugin))
		if len(missing) > 0 {
			errMsg += fmt.Sprintf("; missing plugins: [%s]", strings.Join(missing, ", "))
		}
		if len(duplicates) > 0 {
			errMsg += fmt.Sprintf("; duplicate IDs in result: [%s]", strings.Join(duplicates, ", "))
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	memo := make(map[string]int, len(id2plugin))
	visiting := make(map[string]bool, len(id2plugin)) // nodes on the current DFS path
	pathStack := make([]string, 0, len(id2plugin))    // current path, for cycle reporting

	var dfs func(string) (int, error)
	dfs = func(id string) (int, error) {
		if lv, ok := memo[id]; ok {
			return lv, nil
		}

		// Defensive: ResolveDependencies already rejects cycles, but recompute the
		// path here so a regression produces a precise error instead of recursing.
		if visiting[id] {
			cycleStart := -1
			for i, nodeID := range pathStack {
				if nodeID == id {
					cycleStart = i
					break
				}
			}
			var cyclePath []string
			if cycleStart >= 0 {
				cyclePath = append(cyclePath, pathStack[cycleStart:]...)
				cyclePath = append(cyclePath, id) // close the cycle
			} else {
				cyclePath = append(cyclePath, pathStack...)
				cyclePath = append(cyclePath, id)
			}

			var cycleNames []string
			for _, nodeID := range cyclePath {
				if p, ok := id2plugin[nodeID]; ok {
					cycleNames = append(cycleNames, fmt.Sprintf("%s(%s)", p.Name(), nodeID))
				} else {
					cycleNames = append(cycleNames, nodeID)
				}
			}

			return 0, fmt.Errorf("circular dependency detected in level calculation: %s",
				strings.Join(cycleNames, " -> "))
		}

		p, ok := id2plugin[id]
		if !ok {
			return 0, fmt.Errorf("plugin %s not found in plugin set during level calculation", id)
		}

		visiting[id] = true
		pathStack = append(pathStack, id)
		defer func() {
			visiting[id] = false
			// Remove this id by search rather than a plain pop, since recursive
			// calls may have left the stack in a different shape.
			for i := len(pathStack) - 1; i >= 0; i-- {
				if pathStack[i] == id {
					pathStack = append(pathStack[:i], pathStack[i+1:]...)
					break
				}
			}
		}()

		// Level = 1 + max level of required dependencies.
		best := 0
		for _, depID := range requiredDeps[id] {
			if _, ok := id2plugin[depID]; !ok {
				return 0, fmt.Errorf("dependency %s of plugin %s (%s) not found in plugin set",
					depID, id, p.Name())
			}

			lv, err := dfs(depID)
			if err != nil {
				return 0, err
			}
			if lv+1 > best {
				best = lv + 1
			}
		}

		memo[id] = best
		return best, nil
	}

	result := make([]PluginWithLevel, 0, len(id2plugin))
	for _, id := range orderedIDs {
		p, ok := id2plugin[id]
		if !ok || p == nil {
			return nil, fmt.Errorf("plugin %s not found in id2plugin map after completeness check", id)
		}

		level, err := dfs(id)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate level for plugin %s (%s): %w",
				p.Name(), id, err)
		}

		result = append(result, PluginWithLevel{Plugin: p, level: level})
	}

	if len(result) != len(id2plugin) {
		return nil, fmt.Errorf("result size mismatch: %d plugins in result, %d expected",
			len(result), len(id2plugin))
	}

	return result, nil
}

func resolveRequiredDependencyID(
	id2plugin map[string]plugins.Plugin,
	name2plugin map[string]plugins.Plugin,
	dep plugins.Dependency,
) (string, bool) {
	if dep.ID != "" {
		if _, ok := id2plugin[dep.ID]; ok {
			return dep.ID, true
		}
		if p, ok := name2plugin[dep.ID]; ok && p != nil {
			return p.ID(), true
		}
	}
	if dep.Name != "" {
		if p, ok := name2plugin[dep.Name]; ok && p != nil {
			return p.ID(), true
		}
	}
	return "", false
}

func dependencyDisplayName(dep plugins.Dependency) string {
	switch {
	case dep.Name != "":
		return dep.Name
	case dep.ID != "":
		return dep.ID
	default:
		return "<unknown>"
	}
}

// UnloadOrder returns a best-effort unload order (dependents first, then dependencies).
// Only considers required dependencies that exist in plugs; ignores missing deps.
// Used when TopologicalSort fails so unload order is still dependency-aware.
// Builds load-order graph (dep -> dependent), runs Kahn; unload order = reverse(load order).
func (m *DefaultPluginManager[T]) UnloadOrder(plugs []plugins.Plugin) []plugins.Plugin {
	if len(plugs) == 0 {
		return nil
	}
	id2plugin := make(map[string]plugins.Plugin)
	for _, p := range plugs {
		if p == nil {
			continue
		}
		id2plugin[p.ID()] = p
	}
	if len(id2plugin) == 0 {
		return plugs
	}
	// Graph: dependency -> dependents. Load order = deps first; unload = reverse.
	graph := make(map[string][]string)
	inDegree := make(map[string]int)
	for id := range id2plugin {
		inDegree[id] = 0
	}
	for _, p := range id2plugin {
		deps := p.GetDependencies()
		for _, dep := range deps {
			if dep.Type != plugins.DependencyTypeRequired {
				continue
			}
			if _, ok := id2plugin[dep.ID]; !ok {
				continue
			}
			graph[dep.ID] = append(graph[dep.ID], p.ID())
			inDegree[p.ID()]++
		}
	}
	var loadOrder []string
	queue := make([]string, 0)
	for id, d := range inDegree {
		if d == 0 {
			queue = append(queue, id)
		}
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		loadOrder = append(loadOrder, current)
		for _, next := range graph[current] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	// Append any remaining (cycle or disconnected) in deterministic order
	for id, d := range inDegree {
		if d > 0 {
			loadOrder = append(loadOrder, id)
		}
	}
	// Unload order = reverse load order (dependents first)
	unloadIDs := make([]string, 0, len(loadOrder))
	for i := len(loadOrder) - 1; i >= 0; i-- {
		unloadIDs = append(unloadIDs, loadOrder[i])
	}
	// Map back to plugins; preserve order
	order := make([]plugins.Plugin, 0, len(unloadIDs))
	for _, id := range unloadIDs {
		if p, ok := id2plugin[id]; ok {
			order = append(order, p)
		}
	}
	return order
}
