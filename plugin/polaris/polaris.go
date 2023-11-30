package polaris

import (
	"fmt"
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugin"
	"github.com/go-lynx/lynx/plugin/polaris/conf"
	"github.com/polarismesh/polaris-go/api"
)

var name = "polaris"

type PlugPolaris struct {
	polaris polaris.Polaris
	conf    *conf.Polaris
	weight  int
}

func (p *PlugPolaris) Weight() int {
	return p.weight
}

func (p *PlugPolaris) Name() string {
	return name
}

func (p *PlugPolaris) Load(base interface{}) (plugin.Plugin, error) {
	c, ok := base.(*conf.Polaris)
	if !ok {
		return nil, fmt.Errorf("invalid c type, expected conf.Polaris")
	}
	sdk, err := api.InitContextByConfig(api.NewConfiguration())
	if err != nil {
		app.Lynx().GetHelper().Error(err)
		panic(err)
	}

	p.polaris = polaris.New(
		sdk,
		polaris.WithService(app.Name()),
		polaris.WithNamespace(c.Namespace),
	)

	// set polaris plane for lynx
	app.Lynx().SetControlPlane(p)
	app.Lynx().PlugManager().LoadSpecificPlugins(
		app.Lynx().PlugManager().PreparePlug(
			app.Lynx().GetBootConfiguration(),
		),
	)
	return p, nil
}

func (p *PlugPolaris) Unload() error {
	return nil
}

func Polaris() plugin.Plugin {
	return &PlugPolaris{
		weight: 999999,
	}
}
