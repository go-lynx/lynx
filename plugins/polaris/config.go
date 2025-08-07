package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/config"
)

// ConfigAdapter 配置适配器
// 职责：提供 Polaris 配置中心的相关功能

// GetConfig 从 Polaris 配置中心获取配置
// 该方法会根据传入的配置文件名和配置文件组名，从 Polaris 配置中心获取对应的配置源
func (p *PlugPolaris) GetConfig(fileName string, group string) (config.Source, error) {
	return GetPolaris().Config(
		polaris.WithConfigFile(
			polaris.File{
				Name:  fileName,
				Group: group,
			}))
}
