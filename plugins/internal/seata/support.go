package seata

import (
	"github.com/go-kratos/kratos/v2/config"
)

func (g *SeataClient) Weight() int {
	return g.weight
}

func (g *SeataClient) Name() string {
	return name
}

func (g *SeataClient) DependsOn(b config.Value) []string {
	return nil
}

func (g *SeataClient) ConfPrefix() string {
	return confPrefix
}
