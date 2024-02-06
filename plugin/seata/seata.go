package seata

import (
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugin"
	"github.com/go-lynx/lynx/plugin/seata/conf"
	"github.com/seata/seata-go/pkg/client"
)

var (
	name       = "seata"
	confPrefix = "lynx.seata"
)

type Option func(g *SeataClient)

func Weight(w int) Option {
	return func(g *SeataClient) {
		g.weight = w
	}
}

func Config(c *conf.Seata) Option {
	return func(g *SeataClient) {
		g.conf = c
	}
}

type SeataClient struct {
	conf   *conf.Seata
	weight int
}

func (s *SeataClient) Load(b config.Value) (plugin.Plugin, error) {
	err := b.Scan(s.conf)
	if err != nil {
		return nil, err
	}
	app.Lynx().Helper().Infof("Initializing Seata")
	if s.conf.GetEnabled() {
		client.InitPath(s.conf.GetConfigPath())
	}
	app.Lynx().Helper().Infof("Seata successfully initialized")
	return s, nil
}

func (s *SeataClient) Unload() error {
	return nil
}

func Seata(opts ...Option) plugin.Plugin {
	s := &SeataClient{
		weight: 500,
		conf:   &conf.Seata{},
	}
	for _, option := range opts {
		option(s)
	}
	return s
}
