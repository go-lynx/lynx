package plugins

// Dependency describes a dependency relationship between plugins
// Defines requirements and relationships between plugins
// Dependency 描述了插件之间的依赖关系。
// 定义了插件之间的需求和关系。
type Dependency struct {
	ID       string            // Unique identifier of the required plugin // 所需插件的唯一标识符
	Required bool              // Whether this dependency is mandatory // 此依赖项是否为必需项
	Checker  DependencyChecker // Validates dependency requirements // 验证依赖项要求
	Metadata map[string]any    // Additional dependency information // 额外的依赖项信息
}

// DependencyChecker defines the interface for dependency validation
// Validates plugin dependencies and their conditions
// DependencyChecker 定义了依赖项验证的接口。
// 验证插件依赖项及其条件。
type DependencyChecker interface {
	// Check validates if the dependency condition is met
	// Returns true if the dependency is satisfied
	// Check 验证依赖项条件是否满足。
	// 如果依赖项满足，则返回 true。
	Check(plugin Plugin) bool

	// Description returns a human-readable description of the condition
	// Explains what the dependency checker validates
	// Description 返回条件的易读描述。
	// 解释依赖项检查器验证的内容。
	Description() string
}
