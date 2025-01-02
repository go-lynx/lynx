package polaris

import "github.com/go-kratos/kratos/v2/config"

func (p *PlugPolaris) Weight() int {
	return p.weight
}

func (p *PlugPolaris) Name() string {
	return name
}

func (p *PlugPolaris) DependsOn(config.Value) []string {
	return nil
}

func (p *PlugPolaris) ConfPrefix() string {
	return confPrefix
}
