package cert

import (
	_ "database/sql"
	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugin"
	"github.com/go-lynx/lynx/plugin/cert/conf"
)

var (
	name       = "cert"
	confPrefix = "lynx.application.tls"
)

type PlugCert struct {
	dri    *sql.Driver
	tls    *conf.Tls
	cert   *conf.Cert
	weight int
}

func (ce *PlugCert) GetCrt() []byte {
	return []byte(ce.cert.GetCrt())
}

func (ce *PlugCert) GetKey() []byte {
	return []byte(ce.cert.GetKey())
}

func (ce *PlugCert) GetRootCA() []byte {
	return []byte(ce.cert.GetRootCA())
}

type Option func(ce *PlugCert)

func Weight(w int) Option {
	return func(ce *PlugCert) {
		ce.weight = w
	}
}

func Config(tls *conf.Tls) Option {
	return func(ce *PlugCert) {
		ce.tls = tls
	}
}

func (ce *PlugCert) Load(b config.Value) (plugin.Plugin, error) {
	err := b.Scan(ce.tls)
	if err != nil {
		return nil, err
	}
	app.Lynx().Helper().Infof("Application Certificate Loading")

	source, err := app.Lynx().ControlPlane().Config(ce.tls.GetFileName(), ce.tls.GetGroup())
	if err != nil {
		return nil, err
	}
	c := config.New(config.WithSource(source))
	if err := c.Load(); err != nil {
		return nil, err
	}

	err = c.Scan(ce.cert)
	if err != nil {
		return nil, err
	}

	app.Lynx().SetCert(ce)
	app.Lynx().Helper().Infof("Application Certificate Loaded successfully")
	return ce, nil
}

func (ce *PlugCert) Unload() error {
	return nil
}

func Cert(opts ...Option) plugin.Plugin {
	c := &PlugCert{
		weight: 100,
		tls:    &conf.Tls{},
		cert:   &conf.Cert{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
