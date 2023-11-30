package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/config"
)

func (p *PlugPolaris) Config(fileName string, group string) (config.Source, error) {
	return GetPolaris().Config(polaris.WithConfigFile(polaris.File{
		Name:  fileName,
		Group: group,
	}))
}
