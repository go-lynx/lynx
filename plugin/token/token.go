package token

import (
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugin"
	"github.com/go-lynx/lynx/plugin/token/conf"
)

var (
	name         = "token"
	configPrefix = "lynx.token"
)

type PlugToken struct {
	load   []LoaderToken
	conf   *conf.Token
	weight int
}

func (t *PlugToken) Weight() int {
	return t.weight
}

func (t *PlugToken) Name() string {
	return name
}

func (t *PlugToken) ConfigPrefix() string {
	return configPrefix
}

func (t *PlugToken) Load(b config.Value) (plugin.Plugin, error) {
	err := b.Scan(t.conf)
	if err != nil {
		return nil, err
	}

	app.Lynx().Helper().Infof("Initializing service load")
	source, err := app.Lynx().ControlPlane().Config(t.conf.GetFileName(), t.conf.GetGroup())
	if err != nil {
		return nil, err
	}

	c := config.New(config.WithSource(source))
	if err := c.Load(); err != nil {
		return nil, err
	}

	if t.load != nil && len(t.load) > 0 {
		for _, l := range t.load {
			err := l.Init(c)
			if err != nil {
				return nil, err
			}
		}
	}
	app.Lynx().Helper().Infof("Service load successfully initialized")
	return t, nil
}

func (t *PlugToken) Unload() error {
	return nil
}

func Token(token ...LoaderToken) plugin.Plugin {
	return &PlugToken{
		weight: 0,
		load:   token,
		conf:   &conf.Token{},
	}
}

type LoaderToken interface {
	Init(conf config.Config) error
}
