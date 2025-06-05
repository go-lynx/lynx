package plugins

import (
	"fmt"
	"regexp"
	"strings"
)

// ID format constants
// ID 格式常量
const (
	// DefaultOrg is the default organization identifier
	// DefaultOrg 是默认的组织标识符
	DefaultOrg = "go-lynx"
	// ComponentType represents the plugin component type
	// ComponentType 表示插件组件类型
	ComponentType = "plugin"
)

// IDFormat represents the components of a plugin ID
// IDFormat 表示插件 ID 的各个组成部分
type IDFormat struct {
	Organization string // e.g., "go-lynx" // 例如，"go-lynx"
	Type         string // e.g., "plugin" // 例如，"plugin"
	Name         string // e.g., "http" // 例如，"http"
	Version      string // e.g., "v1" or "v1.0.0" // 例如，"v1" 或 "v1.0.0"
}

// ParsePluginID parses a plugin ID string into its components
// ParsePluginID 将插件 ID 字符串解析为其各个组成部分
func ParsePluginID(id string) (*IDFormat, error) {
	// 使用点号分割插件 ID 字符串
	parts := strings.Split(id, ".")
	// 检查分割后的部分数量是否为 4，如果不是则返回无效插件 ID 错误
	if len(parts) != 4 {
		return nil, ErrInvalidPluginID
	}

	// 初始化 IDFormat 结构体
	format := &IDFormat{
		Organization: parts[0],
		Type:         parts[1],
		Name:         parts[2],
		Version:      parts[3],
	}

	// 验证插件 ID 格式是否正确
	if err := ValidatePluginID(id); err != nil {
		return nil, err
	}

	return format, nil
}

// GeneratePluginID generates a standard format plugin ID
// GeneratePluginID 生成标准格式的插件 ID
func GeneratePluginID(org, name, version string) string {
	// 如果组织名称为空，则使用默认组织名称
	if org == "" {
		org = DefaultOrg
	}

	// 确保版本号以 'v' 开头，如果不是则添加 'v'
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}

	// 按照标准格式拼接插件 ID
	return fmt.Sprintf("%s.%s.%s.%s", org, ComponentType, name, version)
}

// ValidatePluginID validates the format of a plugin ID
// ValidatePluginID 验证插件 ID 的格式
func ValidatePluginID(id string) error {
	// 正则表达式模式解释：
	// ^                     字符串开始
	// [\w-]+               组织名称（单词字符和连字符）
	// \.plugin\.           字面量 ".plugin."
	// [a-z0-9-]+           插件名称（小写字母、数字、连字符）
	// \.v\d+               以 'v' 开头的版本号
	// (?:\.\d+\.\d+)?      可选的补丁版本号（例如，.0.0）
	// $                    字符串结束
	pattern := `^[\w-]+\.plugin\.[a-z0-9-]+\.v\d+(?:\.\d+\.\d+)?$`

	// 使用正则表达式匹配插件 ID
	match, _ := regexp.MatchString(pattern, id)
	if !match {
		return ErrInvalidPluginID
	}
	return nil
}

// GetPluginMainVersion extracts the main version number from a plugin ID
// GetPluginMainVersion 从插件 ID 中提取主版本号
func GetPluginMainVersion(id string) (string, error) {
	// 解析插件 ID
	format, err := ParsePluginID(id)
	if err != nil {
		return "", err
	}

	// 按点号分割版本号，提取主版本号（例如从 v1.0.0 中提取 v1）
	parts := strings.Split(format.Version, ".")
	return parts[0], nil
}

// IsPluginVersionCompatible checks if two plugin versions are compatible
// IsPluginVersionCompatible 检查两个插件版本是否兼容
func IsPluginVersionCompatible(v1, v2 string) bool {
	// 获取两个插件版本的主版本号
	v1Main, err1 := GetPluginMainVersion(v1)
	v2Main, err2 := GetPluginMainVersion(v2)

	// 如果获取主版本号过程中出现错误，则认为版本不兼容
	if err1 != nil || err2 != nil {
		return false
	}

	// 比较两个主版本号是否相同
	return v1Main == v2Main
}
