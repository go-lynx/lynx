package token

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/boot"
	"github.com/go-lynx/lynx/conf"
	"github.com/go-lynx/lynx/plug"
)

var plugName = "token"

type PlugToken struct {
	t      conf.Token
	l      []LoaderToken
	weight int
}

func (t *PlugToken) Weight() int {
	return t.weight
}

func (t *PlugToken) Name() string {
	return plugName
}

func (t *PlugToken) Load(b *conf.Bootstrap) (plug.Plug, error) {
	boot.GetHelper().Infof("Initializing service token")

	source, err := boot.Polaris().Config(polaris.WithConfigFile(polaris.File{
		Name:  "token.yaml",
		Group: "common",
	}))

	if err != nil {
		return nil, err
	}

	c := config.New(
		config.WithSource(source),
	)

	if err := c.Load(); err != nil {
		return nil, err
	}

	if err := c.Scan(&t.t); err != nil {
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

	boot.GetHelper().Infof("Service token successfully initialized")
	return t, nil
}

func (t *PlugToken) Unload() error {
	return nil
}

func GetName() string {
	return plugName
}

func Token(l ...LoaderToken) plug.Plug {
	return &PlugToken{
		weight: 0,
		l:      l,
	}
}

type LoaderToken interface {
	Init(Token *conf.Token) error
}
