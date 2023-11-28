package token

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugin"
	polaris2 "github.com/go-lynx/lynx/plugin/polaris"
	"github.com/go-lynx/lynx/plugin/token/conf"
)

var plugName = "token"

type PlugToken struct {
	t      conf.Jtw
	l      []LoaderToken
	weight int
}

func (t *PlugToken) Weight() int {
	return t.weight
}

func (t *PlugToken) Name() string {
	return plugName
}

func (t *PlugToken) Load(_ interface{}) (plugin.Plugin, error) {
	app.GetHelper().Infof("Initializing service token")

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

	if err := s.Scan(&t.t); err != nil {
		return nil, err
	}

	if t.l != nil && len(t.l) > 0 {
		for _, l := range t.l {
			err := l.Init(&t.t)
			if err != nil {
				return nil, err
			}
		}
	}

	app.GetHelper().Infof("Service token successfully initialized")
	return t, nil
}

func (t *PlugToken) Unload() error {
	return nil
}

func GetName() string {
	return plugName
}

func Token(l ...LoaderToken) plugin.Plugin {
	return &PlugToken{
		weight: 0,
		l:      l,
	}
}

type LoaderToken interface {
	Init(Token *conf.Jtw) error
}
