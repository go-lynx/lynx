package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugin"
	"github.com/go-lynx/lynx/plugin/polaris/conf"
	"github.com/polarismesh/polaris-go/api"
)

var (
	name       = "polaris"
	confPrefix = "lynx.polaris"
)

type PlugPolaris struct {
	polaris *polaris.Polaris
	conf    *conf.Polaris
	weight  int
}

type Option func(h *PlugPolaris)

func Weight(w int) Option {
	return func(h *PlugPolaris) {
		h.weight = w
	}
}

func Config(c *conf.Polaris) Option {
	return func(p *PlugPolaris) {
		p.conf = c
	}
}

func (p *PlugPolaris) Load(b config.Value) (plugin.Plugin, error) {
	err := b.Scan(p.conf)
	if err != nil {
		return nil, err
	}

	sdk, err := api.InitContextByConfig(api.NewConfiguration())
	if err != nil {
		app.Lynx().Helper().Error(err)
		panic(err)
	}

	polar := polaris.New(
		sdk,
		polaris.WithService(app.Name()),
		polaris.WithNamespace(p.conf.Namespace),
	)
	p.polaris = &polar

	// set polaris plane for lynx
	app.Lynx().SetControlPlane(p)
	configuration := app.Lynx().ControlPlaneBootConfiguration()
	plugins := app.Lynx().PlugManager().PreparePlug(configuration)
	app.Lynx().PlugManager().LoadPluginsByName(
		plugins,
		configuration,
	)
	return p, nil
}

func (p *PlugPolaris) Unload() error {
	return nil
}

func Polaris() plugin.Plugin {
	return &PlugPolaris{
		weight: 999999,
		conf:   &conf.Polaris{},
	}
}
