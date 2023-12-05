package token

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugin"
	polaris2 "github.com/go-lynx/lynx/plugin/polaris"
	"github.com/go-lynx/lynx/plugin/token/conf"
)

var name = "token"

type PlugToken struct {
	conf   *conf.Jtw
	token  []LoaderToken
	weight int
}

func (t *PlugToken) Weight() int {
	return t.weight
}

func (t *PlugToken) Name() string {
	return name
}

func (t *PlugToken) Load(b config.Value) (plugin.Plugin, error) {
	err := b.Scan(t.conf)
	if err != nil {
		return nil, err
	}

	app.Lynx().GetHelper().Infof("Initializing service token")

	source, err := polaris2.GetPolaris().Config(polaris.WithConfigFile(polaris.File{
		Name:  "token.yaml",
		Group: "common",
	}))

	if err != nil {
		return nil, err
	}

	s := config.New(
		config.WithSource(source),
	)

	if err := s.Load(); err != nil {
		return nil, err
	}

	if err := s.Scan(&t.conf); err != nil {
		return nil, err
	}

	if t.token != nil && len(t.token) > 0 {
		for _, l := range t.token {
			err := l.Init(t.conf)
			if err != nil {
				return nil, err
			}
		}
	}

	app.Lynx().GetHelper().Infof("Service token successfully initialized")
	return t, nil
}

func (t *PlugToken) Unload() error {
	return nil
}

func GetName() string {
	return name
}

func Token(token ...LoaderToken) plugin.Plugin {
	return &PlugToken{
		weight: 0,
		token:  token,
		conf:   &conf.Jtw{},
	}
}

type LoaderToken interface {
	Init(Token *conf.Jtw) error
}
