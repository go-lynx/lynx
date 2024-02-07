package app

import (
	"github.com/go-kratos/kratos/v2/config"
)

// PreparePlug Bootstrap plugin loading through remote or local configuration files
func (m *DefaultLynxPluginManager) PreparePlug(config config.Config) []string {
	table := m.factory.GetRegisterTable()
	var plugNames = make([]string, 0)
	for confPrefix := range table {
		value := config.Value(confPrefix)
		if value.Load() == nil {
			continue
		}

		names := table[confPrefix]
		if len(names) == 0 {
			continue
		}

		for _, name := range names {
			if _, exists := m.pluginMap[name]; !exists && m.factory.Exists(name) {
				p, err := m.factory.CreateByName(name)
				if err != nil {
					Lynx().Helper().Errorf("Plugin factory load error: %v", err)
					panic(err)
				}
				m.pluginList = append(m.pluginList, p)
				m.pluginMap[p.Name()] = p
				plugNames = append(plugNames, name)
			}
		}
	}
	return plugNames
}
