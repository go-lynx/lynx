package http

import (
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/plugin/cert"
	"github.com/go-lynx/lynx/plugin/http/conf"
)

func (h *ServiceHttp) Name() string {
	return name
}

func (h *ServiceHttp) DependsOn(b config.Value) []string {
	if b == nil {
		return nil
	}
	var c conf.Http
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

func (h *ServiceHttp) ConfPrefix() string {
	return confPrefix
}

func (h *ServiceHttp) Weight() int {
	return h.weight
}
