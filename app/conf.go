package app

import (
	"github.com/go-kratos/kratos/v2/config"
)

// PreparePlug Bootstrap plugin loading through remote or local configuration files
func (m *DefaultLynxPluginManager) PreparePlug(config config.Config) []string {
	// 获取所有已注册插件的配置前缀列表
	table := m.factory.GetRegisterTable()
	// 初始化一个字符串切片，用于存储将要加载的插件名称
	var plugNames = make([]string, 0)

	// 遍历配置前缀列表
	for confPrefix := range table {
		// 尝试从配置中获取对应前缀的值
		value := config.Value(confPrefix)
		// 如果获取失败（即配置中不存在该前缀），则跳过当前循环
		if value.Load() == nil {
			continue
		}

		// 获取与当前配置前缀关联的插件名称列表
		names := table[confPrefix]
		// 如果名称列表为空，则跳过当前循环
		if len(names) == 0 {
			continue
		}

		// 遍历名称列表
		for _, name := range names {
			// 检查插件是否已经存在于插件管理器中
			if _, exists := m.pluginMap[name]; !exists && m.factory.Exists(name) {
				// 如果插件不存在，则尝试从工厂中创建该插件
				p, err := m.factory.CreateByName(name)
				// 如果创建过程中发生错误，记录错误并抛出 panic
				if err != nil {
					Lynx().Helper().Errorf("Plugin factory load error: %v", err)
					panic(err)
				}
				// 将新创建的插件添加到插件管理器的插件列表中
				m.pluginList = append(m.pluginList, p)
				// 将新创建的插件添加到插件管理器的插件映射中
				m.pluginMap[p.Name()] = p
				// 将新创建的插件名称添加到将要加载的插件名称列表中
				plugNames = append(plugNames, name)
			}
		}
	}
	// 返回将要加载的插件名称列表
	return plugNames
}
