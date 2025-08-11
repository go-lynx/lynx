package app

import (
	"fmt"

	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"

	"github.com/go-kratos/kratos/v2/config"
)

// PreparePlug 方法通过远程或本地配置文件引导插件加载。
// 它基于配置处理插件的初始化和注册操作。
// 返回一个成功准备好的插件列表和错误信息。
func (m *DefaultPluginManager) PreparePlug(config config.Config) ([]plugins.Plugin, error) {
	// 检查配置是否为 nil，如果为 nil 则记录错误日志并返回 nil
	if config == nil {
		log.Error("Configuration is nil")
		return nil, fmt.Errorf("configuration is nil")
	}

	// 获取包含所有已注册插件配置前缀的注册表
	table := m.factory.GetPluginRegistry()
	// 检查注册表是否为空，如果为空则记录警告日志并返回 nil
	if len(table) == 0 {
		log.Warn("No plugins registered in factory")
		return nil, fmt.Errorf("no plugins registered in factory")
	}

	// 初始化一个切片，用于存储待加载的插件实例，预分配容量为注册表的长度
	prepared := make([]plugins.Plugin, 0, len(table))

	// 遍历配置前缀
	for confPrefix, names := range table {
		// 检查配置前缀是否为空，如果为空则记录警告日志并跳过当前循环
		if confPrefix == "" {
			log.Warnf("Empty configuration prefix found, skipping")
			continue
		}

		// 尝试获取当前前缀对应的配置值
		cfg := config.Value(confPrefix)
		// 检查配置值是否为 nil，如果为 nil 则记录调试日志并跳过当前循环
		if cfg == nil {
			log.Debugf("No configuration found for prefix: %s", confPrefix)
			continue
		}

		// 加载配置值，如果加载结果为 nil 则记录调试日志并跳过当前循环
		if loaded := cfg.Load(); loaded == nil {
			log.Debugf("Configuration cfg is nil for prefix: %s", confPrefix)
			continue
		}

		// 检查是否有与前缀关联的插件名称，如果没有则记录调试日志并跳过当前循环
		if len(names) == 0 {
			log.Debugf("No plugins associated with prefix: %s", confPrefix)
			continue
		}

		// 处理每个插件名称
		for _, name := range names {
			// 检查插件名称是否为空，如果为空则记录警告日志并跳过当前循环
			if name == "" {
				log.Warn("Empty plugin name found, skipping")
				continue
			}

			// 检查插件是否已存在且能否创建
			if err := m.preparePlugin(name); err != nil {
				continue
			}

			// 获取插件实例并添加到切片中
			if value, ok := m.pluginInstances.Load(name); ok {
				if plugin, ok := value.(plugins.Plugin); ok {
					prepared = append(prepared, plugin)
				}
			}
		}
	}

	// 检查是否有成功准备的插件，如果没有则记录警告日志，否则记录成功信息
	if len(prepared) != 0 {
		log.Infof("successfully prepared %d plugins", len(prepared))
	}

	return prepared, nil
}

// preparePlugin 处理单个插件的准备工作。
// 它会检查插件是否已存在，创建插件实例，并将其添加到管理器中。
// 如果任何步骤失败，则返回错误。
func (m *DefaultPluginManager) preparePlugin(name string) error {
	// 检查插件是否已经加载，如果已加载则返回错误信息
	if _, exists := m.pluginInstances.Load(name); exists {
		return fmt.Errorf("plugin %s is already loaded", name)
	}

	// 验证插件是否存在于工厂中，如果不存在则返回错误信息
	if !m.factory.HasPlugin(name) {
		return fmt.Errorf("plugin %s does not exist in factory", name)
	}

	// 创建插件实例，如果创建失败则返回错误信息
	p, err := m.factory.CreatePlugin(name)
	if err != nil {
		return fmt.Errorf("failed to create plugin %s: %v", name, err)
	}

	// 检查创建的插件实例是否为 nil，如果为 nil 则返回错误信息
	if p == nil {
		return fmt.Errorf("created plugin %s is nil", name)
	}

	// 将插件添加到管理器的跟踪结构中
	m.mu.Lock()
	m.pluginList = append(m.pluginList, p)
	m.mu.Unlock()
	m.pluginInstances.Store(p.Name(), p)

	return nil
}
