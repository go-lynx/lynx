package cert

import "github.com/go-kratos/kratos/v2/config"

func (ce *PlugCert) Name() string {
	return name
}

func (ce *PlugCert) DependsOn(config.Value) []string {
	return nil
}

func (ce *PlugCert) Weight() int {
	return ce.weight
}

func (ce *PlugCert) ConfPrefix() string {
	return confPrefix
}

func GetName() string {
	return name
}
