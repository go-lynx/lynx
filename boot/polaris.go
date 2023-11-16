package boot

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-lynx/lynx/conf"
	"github.com/polarismesh/polaris-go/api"
)

var (
	p polaris.Polaris
)

func (a *App) initPolaris(c *conf.Bootstrap) {
	ac := api.NewConfiguration()
	sdk, err := api.InitContextByConfig(ac)
	if err != nil {
		GetHelper().Error(err)
		panic(err)
	}
	p = polaris.New(
		sdk,
		polaris.WithNamespace(c.Server.Polaris.Namespace),
		polaris.WithService(c.Server.Name),
	)
	polarisConfigLoad(c)
}

func polarisConfigLoad(c *conf.Bootstrap) {
	yaml := c.Server.Name + "-" + c.Server.Version + ".yaml"
	dfLog.Infof("Reading from the configuration center,file:[%v] group:[%v] namespace:[%v]", yaml, c.Server.Name, c.Server.Polaris.Namespace)
	source, err := p.Config(polaris.WithConfigFile(polaris.File{
		Name:  yaml,
		Group: c.Server.Name,
	}))
	if err != nil {
		dfLog.Error(err)
		panic(err)
	}
	configLoad(c, source)
}

func Polaris() *polaris.Polaris {
	return &p
}
