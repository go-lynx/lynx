package grpc

import (
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/plugin/cert"
	"github.com/go-lynx/lynx/plugin/grpc/conf"
)

func (g *ServiceGrpc) Weight() int {
	return g.weight
}

func (g *ServiceGrpc) Name() string {
	return name
}

func (g *ServiceGrpc) DependsOn(b config.Value) []string {
	if b == nil {
		return nil
	}
	var c conf.Grpc
	err := b.Scan(&c)
	if err != nil {
		return nil
	}
	// When the configuration file specifies the need to enable TLS, it depends on the certificate plugin
	if c.Tls {
		return []string{cert.GetName()}
	}
	return nil
}

func (g *ServiceGrpc) ConfPrefix() string {
	return confPrefix
}
