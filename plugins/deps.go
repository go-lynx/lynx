package plugins

import (
	"fmt"
	"strings"
	"sync"
)

// DependencyType 定义依赖类型
type DependencyType string

const (
	// DependencyTypeRequired 必需依赖
	DependencyTypeRequired DependencyType = "required"
	// DependencyTypeOptional 可选依赖
	DependencyTypeOptional DependencyType = "optional"
	// DependencyTypeConflicts 冲突依赖
	DependencyTypeConflicts DependencyType = "conflicts"
	// DependencyTypeProvides 提供依赖
	DependencyTypeProvides DependencyType = "provides"
)

// VersionConstraint 版本约束
type VersionConstraint struct {
	MinVersion      string   `json:"min_version"`      // 最小版本
	MaxVersion      string   `json:"max_version"`      // 最大版本
	ExactVersion    string   `json:"exact_version"`    // 精确版本
	ExcludeVersions []string `json:"exclude_versions"` // 排除版本
}

// Dependency 描述插件之间的依赖关系
type Dependency struct {
	ID                string             `json:"id"`                 // 依赖插件的唯一标识符
	Name              string             `json:"name"`               // 依赖插件的名称
	Type              DependencyType     `json:"type"`               // 依赖类型
	VersionConstraint *VersionConstraint `json:"version_constraint"` // 版本约束
	Required          bool               `json:"required"`           // 是否为必需依赖
	Checker           DependencyChecker  `json:"-"`                  // 依赖验证器
	Metadata          map[string]any     `json:"metadata"`           // 额外依赖信息
	Description       string             `json:"description"`        // 依赖描述
}

// DependencyChecker 定义依赖项验证的接口
type DependencyChecker interface {
	// Check 验证依赖项条件是否满足
	Check(plugin Plugin) bool
	// Description 返回条件的易读描述
	Description() string
}

// DependencyManager 依赖管理器接口
type DependencyManager interface {
	// AddDependency 添加依赖关系
	AddDependency(pluginID string, dependency *Dependency) error
	// RemoveDependency 移除依赖关系
	RemoveDependency(pluginID string, dependencyID string) error
	// GetDependencies 获取插件的所有依赖
	GetDependencies(pluginID string) []*Dependency
	// GetDependents 获取依赖该插件的所有插件
	GetDependents(pluginID string) []string
	// CheckCircularDependencies 检查循环依赖
	CheckCircularDependencies() ([]string, error)
	// ResolveDependencies 解析依赖关系，返回正确的加载顺序
	ResolveDependencies() ([]string, error)
	// CheckVersionConflicts 检查版本冲突
	CheckVersionConflicts() ([]VersionConflict, error)
	// ValidateDependencies 验证所有依赖是否满足
	ValidateDependencies(plugins map[string]Plugin) ([]DependencyError, error)
}

// VersionConflict 版本冲突信息
type VersionConflict struct {
	PluginID         string `json:"plugin_id"`
	DependencyID     string `json:"dependency_id"`
	RequiredVersion  string `json:"required_version"`
	AvailableVersion string `json:"available_version"`
	ConflictType     string `json:"conflict_type"`
	Description      string `json:"description"`
}

// DependencyError 依赖错误信息
type DependencyError struct {
	PluginID     string `json:"plugin_id"`
	DependencyID string `json:"dependency_id"`
	ErrorType    string `json:"error_type"`
	Message      string `json:"message"`
	Severity     string `json:"severity"` // "error", "warning", "info"
}

// DependencyGraph 依赖图结构
type DependencyGraph struct {
	// 插件ID -> 依赖列表
	dependencies map[string][]*Dependency
	// 插件ID -> 被依赖的插件列表
	dependents map[string][]string
	// 插件ID -> 插件信息
	plugins map[string]Plugin
	// 互斥锁
	mu sync.RWMutex
}

// NewDependencyGraph 创建新的依赖图
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		dependencies: make(map[string][]*Dependency),
		dependents:   make(map[string][]string),
		plugins:      make(map[string]Plugin),
	}
}

// AddPlugin 添加插件到依赖图
func (dg *DependencyGraph) AddPlugin(plugin Plugin) {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	pluginID := plugin.ID()
	dg.plugins[pluginID] = plugin
}

// RemovePlugin 从依赖图中移除插件
func (dg *DependencyGraph) RemovePlugin(pluginID string) {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	delete(dg.plugins, pluginID)
	delete(dg.dependencies, pluginID)

	// 从所有被依赖列表中移除
	for _, deps := range dg.dependents {
		for i, dep := range deps {
			if dep == pluginID {
				deps = append(deps[:i], deps[i+1:]...)
				break
			}
		}
	}
}

// AddDependency 添加依赖关系
func (dg *DependencyGraph) AddDependency(pluginID string, dependency *Dependency) error {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	// 检查插件是否存在
	if _, exists := dg.plugins[pluginID]; !exists {
		return fmt.Errorf("plugin %s not found", pluginID)
	}

	// 检查依赖插件是否存在
	if _, exists := dg.plugins[dependency.ID]; !exists {
		return fmt.Errorf("dependency plugin %s not found", dependency.ID)
	}

	// 添加依赖关系
	dg.dependencies[pluginID] = append(dg.dependencies[pluginID], dependency)

	// 更新被依赖关系
	dg.dependents[dependency.ID] = append(dg.dependents[dependency.ID], pluginID)

	return nil
}

// RemoveDependency 移除依赖关系
func (dg *DependencyGraph) RemoveDependency(pluginID string, dependencyID string) error {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	deps, exists := dg.dependencies[pluginID]
	if !exists {
		return fmt.Errorf("plugin %s has no dependencies", pluginID)
	}

	// 查找并移除依赖
	for i, dep := range deps {
		if dep.ID == dependencyID {
			dg.dependencies[pluginID] = append(deps[:i], deps[i+1:]...)

			// 更新被依赖关系
			if dependents, ok := dg.dependents[dependencyID]; ok {
				for j, dep := range dependents {
					if dep == pluginID {
						dg.dependents[dependencyID] = append(dependents[:j], dependents[j+1:]...)
						break
					}
				}
			}
			return nil
		}
	}

	return fmt.Errorf("dependency %s not found for plugin %s", dependencyID, pluginID)
}

// GetDependencies 获取插件的所有依赖
func (dg *DependencyGraph) GetDependencies(pluginID string) []*Dependency {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	if deps, exists := dg.dependencies[pluginID]; exists {
		result := make([]*Dependency, len(deps))
		copy(result, deps)
		return result
	}
	return nil
}

// GetDependents 获取依赖该插件的所有插件
func (dg *DependencyGraph) GetDependents(pluginID string) []string {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	if deps, exists := dg.dependents[pluginID]; exists {
		result := make([]string, len(deps))
		copy(result, deps)
		return result
	}

	return nil
}

// CheckCircularDependencies 检查循环依赖
func (dg *DependencyGraph) CheckCircularDependencies() ([]string, error) {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	cycle := make([]string, 0)

	// 深度优先搜索检测循环
	var dfs func(pluginID string) bool
	dfs = func(pluginID string) bool {
		if recStack[pluginID] {
			// 找到循环依赖
			cycle = append(cycle, pluginID)
			return true
		}

		if visited[pluginID] {
			return false
		}

		visited[pluginID] = true
		recStack[pluginID] = true

		deps := dg.dependencies[pluginID]
		for _, dep := range deps {
			if dep.Type == DependencyTypeRequired {
				if dfs(dep.ID) {
					if len(cycle) == 1 || cycle[len(cycle)-1] != pluginID {
						cycle = append(cycle, pluginID)
					}
					return true
				}
			}
		}

		recStack[pluginID] = false
		return false
	}

	// 检查所有插件
	for id := range dg.plugins {
		if !visited[id] {
			if dfs(id) {
				// 反转循环路径
				for i, j := 0, len(cycle)-1; i < j; i, j = i+1, j-1 {
					cycle[i], cycle[j] = cycle[j], cycle[i]
				}
				return cycle, fmt.Errorf("circular dependency detected: %s", strings.Join(cycle, " -> "))
			}
		}
	}

	return nil, nil
}

// ResolveDependencies 解析依赖关系，返回正确的加载顺序
func (dg *DependencyGraph) ResolveDependencies() ([]string, error) {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	// 首先检查循环依赖
	if _, err := dg.CheckCircularDependencies(); err != nil {
		return nil, err
	}

	// 拓扑排序
	inDegree := make(map[string]int)
	graph := make(map[string][]string)

	// 初始化入度
	for pluginID := range dg.plugins {
		inDegree[pluginID] = 0
	}

	// 构建图和计算入度
	for pluginID, deps := range dg.dependencies {
		for _, dep := range deps {
			if dep.Type == DependencyTypeRequired {
				graph[pluginID] = append(graph[pluginID], dep.ID)
				inDegree[dep.ID]++
			}
		}
	}

	// 拓扑排序
	var result []string
	queue := make([]string, 0)

	// 找到所有入度为0的节点
	for pluginID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, pluginID)
		}
	}

	// 处理队列
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// 更新相关节点的入度
		for _, dep := range graph[current] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	// 检查是否所有节点都被处理
	if len(result) != len(dg.plugins) {
		return nil, fmt.Errorf("dependency resolution failed: some plugins have unresolved dependencies")
	}

	return result, nil
}

// CheckVersionConflicts 检查版本冲突
func (dg *DependencyGraph) CheckVersionConflicts() ([]VersionConflict, error) {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	var conflicts []VersionConflict

	for pluginID, deps := range dg.dependencies {
		plugin := dg.plugins[pluginID]
		if plugin == nil {
			continue
		}

		for _, dep := range deps {
			if dep.VersionConstraint == nil {
				continue
			}

			depPlugin := dg.plugins[dep.ID]
			if depPlugin == nil {
				continue
			}

			// 检查版本约束
			if conflict := dg.checkVersionConstraint(plugin, dep, depPlugin); conflict != nil {
				conflicts = append(conflicts, *conflict)
			}
		}
	}

	return conflicts, nil
}

// checkVersionConstraint 检查单个版本约束
func (dg *DependencyGraph) checkVersionConstraint(plugin Plugin, dep *Dependency, depPlugin Plugin) *VersionConflict {
	constraint := dep.VersionConstraint
	depVersion := depPlugin.Version()

	// 检查精确版本
	if constraint.ExactVersion != "" && constraint.ExactVersion != depVersion {
		return &VersionConflict{
			PluginID:         plugin.ID(),
			DependencyID:     dep.ID,
			RequiredVersion:  constraint.ExactVersion,
			AvailableVersion: depVersion,
			ConflictType:     "exact_version_mismatch",
			Description: fmt.Sprintf("Plugin %s requires exact version %s of %s, but %s is available",
				plugin.ID(), constraint.ExactVersion, dep.ID, depVersion),
		}
	}

	// 检查排除版本
	for _, excludedVersion := range constraint.ExcludeVersions {
		if excludedVersion == depVersion {
			return &VersionConflict{
				PluginID:         plugin.ID(),
				DependencyID:     dep.ID,
				RequiredVersion:  "any version except " + excludedVersion,
				AvailableVersion: depVersion,
				ConflictType:     "excluded_version",
				Description: fmt.Sprintf("Plugin %s excludes version %s of %s",
					plugin.ID(), excludedVersion, dep.ID),
			}
		}
	}

	// 检查版本范围（这里需要实现版本比较逻辑）
	// 为了简化，这里只做基本的字符串比较
	if constraint.MinVersion != "" && depVersion < constraint.MinVersion {
		return &VersionConflict{
			PluginID:         plugin.ID(),
			DependencyID:     dep.ID,
			RequiredVersion:  ">= " + constraint.MinVersion,
			AvailableVersion: depVersion,
			ConflictType:     "version_too_low",
			Description: fmt.Sprintf("Plugin %s requires version >= %s of %s, but %s is available",
				plugin.ID(), constraint.MinVersion, dep.ID, depVersion),
		}
	}

	if constraint.MaxVersion != "" && depVersion > constraint.MaxVersion {
		return &VersionConflict{
			PluginID:         plugin.ID(),
			DependencyID:     dep.ID,
			RequiredVersion:  "<= " + constraint.MaxVersion,
			AvailableVersion: depVersion,
			ConflictType:     "version_too_high",
			Description: fmt.Sprintf("Plugin %s requires version <= %s of %s, but %s is available",
				plugin.ID(), constraint.MaxVersion, dep.ID, depVersion),
		}
	}

	return nil
}

// ValidateDependencies 验证所有依赖是否满足
func (dg *DependencyGraph) ValidateDependencies(plugins map[string]Plugin) ([]DependencyError, error) {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	var errors []DependencyError

	for pluginID, deps := range dg.dependencies {
		plugin := dg.plugins[pluginID]
		if plugin == nil {
			continue
		}

		for _, dep := range deps {
			if dep.Type == DependencyTypeRequired {
				// 检查依赖插件是否存在
				depPlugin, exists := plugins[dep.ID]
				if !exists {
					errors = append(errors, DependencyError{
						PluginID:     pluginID,
						DependencyID: dep.ID,
						ErrorType:    "missing_dependency",
						Message:      fmt.Sprintf("Required dependency %s not found", dep.ID),
						Severity:     "error",
					})
					continue
				}

				// 检查版本约束
				if dep.VersionConstraint != nil {
					if conflict := dg.checkVersionConstraint(plugin, dep, depPlugin); conflict != nil {
						errors = append(errors, DependencyError{
							PluginID:     pluginID,
							DependencyID: dep.ID,
							ErrorType:    "version_conflict",
							Message:      conflict.Description,
							Severity:     "error",
						})
					}
				}

				// 检查依赖检查器
				if dep.Checker != nil {
					if !dep.Checker.Check(depPlugin) {
						errors = append(errors, DependencyError{
							PluginID:     pluginID,
							DependencyID: dep.ID,
							ErrorType:    "dependency_check_failed",
							Message:      fmt.Sprintf("Dependency check failed: %s", dep.Checker.Description()),
							Severity:     "error",
						})
					}
				}
			}
		}
	}

	return errors, nil
}

// GetDependencyTree 获取依赖树结构
func (dg *DependencyGraph) GetDependencyTree(pluginID string) map[string]interface{} {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	return dg.buildDependencyTree(pluginID, make(map[string]bool))
}

// buildDependencyTree 递归构建依赖树
func (dg *DependencyGraph) buildDependencyTree(pluginID string, visited map[string]bool) map[string]interface{} {
	if visited[pluginID] {
		return map[string]interface{}{
			"id":       pluginID,
			"circular": true,
		}
	}

	visited[pluginID] = true
	defer delete(visited, pluginID)

	tree := map[string]interface{}{
		"id":           pluginID,
		"dependencies": []map[string]interface{}{},
	}

	if deps, exists := dg.dependencies[pluginID]; exists {
		for _, dep := range deps {
			depTree := dg.buildDependencyTree(dep.ID, visited)
			depInfo := map[string]interface{}{
				"id":          dep.ID,
				"type":        dep.Type,
				"required":    dep.Required,
				"description": dep.Description,
				"subtree":     depTree,
			}

			if dep.VersionConstraint != nil {
				depInfo["version_constraint"] = dep.VersionConstraint
			}

			tree["dependencies"] = append(tree["dependencies"].([]map[string]interface{}), depInfo)
		}
	}

	return tree
}

// GetDependencyStats 获取依赖统计信息
func (dg *DependencyGraph) GetDependencyStats() map[string]interface{} {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	stats := map[string]interface{}{
		"total_plugins":        len(dg.plugins),
		"total_dependencies":   0,
		"required_deps":        0,
		"optional_deps":        0,
		"conflict_deps":        0,
		"plugins_with_deps":    0,
		"plugins_without_deps": 0,
	}

	for _, deps := range dg.dependencies {
		if len(deps) > 0 {
			stats["plugins_with_deps"] = stats["plugins_with_deps"].(int) + 1
			stats["total_dependencies"] = stats["total_dependencies"].(int) + len(deps)

			for _, dep := range deps {
				switch dep.Type {
				case DependencyTypeRequired:
					stats["required_deps"] = stats["required_deps"].(int) + 1
				case DependencyTypeOptional:
					stats["optional_deps"] = stats["optional_deps"].(int) + 1
				case DependencyTypeConflicts:
					stats["conflict_deps"] = stats["conflict_deps"].(int) + 1
				}
			}
		} else {
			stats["plugins_without_deps"] = stats["plugins_without_deps"].(int) + 1
		}
	}

	return stats
}

// HasPlugin 检查插件是否存在
func (dg *DependencyGraph) HasPlugin(pluginID string) bool {
	dg.mu.RLock()
	defer dg.mu.RUnlock()
	_, exists := dg.plugins[pluginID]
	return exists
}

// GetAllDependencies 获取所有插件的依赖关系
func (dg *DependencyGraph) GetAllDependencies() map[string][]*Dependency {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	// 创建副本以避免外部修改
	result := make(map[string][]*Dependency)
	for pluginID, deps := range dg.dependencies {
		depsCopy := make([]*Dependency, len(deps))
		copy(depsCopy, deps)
		result[pluginID] = depsCopy
	}
	return result
}

// GetAllPlugins 获取所有插件
func (dg *DependencyGraph) GetAllPlugins() map[string]Plugin {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	// 创建副本以避免外部修改
	result := make(map[string]Plugin)
	for pluginID, plugin := range dg.plugins {
		result[pluginID] = plugin
	}
	return result
}

// CleanupOrphanedDependencies 清理孤立的依赖关系
func (dg *DependencyGraph) CleanupOrphanedDependencies() int {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	cleaned := 0

	// 检查所有依赖关系，移除指向不存在插件的依赖
	for pluginID, deps := range dg.dependencies {
		validDeps := make([]*Dependency, 0)

		for _, dep := range deps {
			if _, exists := dg.plugins[dep.ID]; exists {
				validDeps = append(validDeps, dep)
			} else {
				cleaned++
			}
		}

		if len(validDeps) != len(deps) {
			dg.dependencies[pluginID] = validDeps
		}
	}

	// 清理被依赖关系
	for depID, dependents := range dg.dependents {
		if _, exists := dg.plugins[depID]; !exists {
			delete(dg.dependents, depID)
			cleaned += len(dependents)
		}
	}

	return cleaned
}
