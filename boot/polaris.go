package boot

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/polarismesh/polaris-go/api"
)

var (
	p polaris.Polaris
)

func (a *App) initPolaris(lynx *Lynx) {
	sdk, err := api.InitContextByConfig(api.NewConfiguration())
	if err != nil {
		GetHelper().Error(err)
		panic(err)
	}

	p = polaris.New(
		sdk,
		polaris.WithService(lynx.Application.Name),
		polaris.WithNamespace(lynx.Polaris.Namespace),
	)

	polarisConfigLoad(lynx)
}

func polarisConfigLoad(lynx *Lynx) {
	fileName := lynx.Application.Name + "-" + lynx.Application.Version + ".fileName"

	dfLog.Infof("Reading from the configuration center,file:[%v] group:[%v] namespace:[%v]",
		fileName,
		lynx.Application.Name,
		lynx.Polaris.Namespace)

	source, err := p.Config(polaris.WithConfigFile(polaris.File{
		Name:  fileName,
		Group: lynx.Application.Name,
	}))
	if err != nil {
		dfLog.Error(err)
		panic(err)
	}

	configLoad(lynx, source)
}

func GetPolaris() *polaris.Polaris {
	return &p
}
