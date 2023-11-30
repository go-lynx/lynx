package app

import (
	"github.com/go-kratos/kratos/v2/config"
)

// PreparePlug Bootstrap plugin loading through remote or local configuration files
func (m *LynxPluginManager) PreparePlug(config map[string]config.Value) []string {
	var plugNames = make([]string, 0)
	for name := range config {
		if _, exists := m.plugMap[name]; !exists && m.factory.Exists(name) {
			p, err := m.factory.Create(name)
			if err != nil {
				Lynx().GetHelper().Errorf("Plugin factory load error: %v", err)
				panic(err)
			}
			m.plugins = append(m.plugins, p)
			m.plugMap[p.Name()] = p
			plugNames = append(plugNames, name)
		}
	}
	return plugNames
}
