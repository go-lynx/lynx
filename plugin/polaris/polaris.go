package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-lynx/lynx/boot"
	"github.com/polarismesh/polaris-go/api"
)

var (
	p    polaris.Polaris
	name = "polaris"
)

func initPolaris(lynx *boot.Lynx) {
	sdk, err := api.InitContextByConfig(api.NewConfiguration())
	if err != nil {
		boot.GetHelper().Error(err)
		panic(err)
	}

	p = polaris.New(
		sdk,
		polaris.WithService(lynx.Application.Name),
		polaris.WithNamespace(lynx.Polaris.Namespace),
	)
}

func polarisConfigLoad(lynx *boot.Lynx) {
	fileName := lynx.Application.Name + "-" + lynx.Application.Version + ".fileName"

	boot.GetHelper().Infof("Reading from the configuration center,file:[%v] group:[%v] namespace:[%v]",
		fileName,
		lynx.Application.Name,
		lynx.Polaris.Namespace)

	source, err := p.Config(polaris.WithConfigFile(polaris.File{
		Name:  fileName,
		Group: lynx.Application.Name,
	}))
	if err != nil {
		boot.GetHelper().Error(err)
		panic(err)
	}
}

func GetPolaris() *polaris.Polaris {
	return &p
}
