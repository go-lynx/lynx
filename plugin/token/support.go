package token

import "github.com/go-kratos/kratos/v2/config"

func (t *PlugToken) Weight() int {
	return t.weight
}

func (t *PlugToken) Name() string {
	return name
}

func (t *PlugToken) DependsOn(config.Value) []string {
	return nil
}

func (t *PlugToken) ConfPrefix() string {
	return confPrefix
}
