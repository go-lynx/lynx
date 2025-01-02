package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/config"
)

// Config 方法用于从 Polaris 配置中心获取配置
func (p *PlugPolaris) Config(fileName string, group string) (config.Source, error) {
	// 调用 GetPolaris() 函数获取 Polaris 实例，并使用 WithConfigFile 方法设置配置文件信息
	return GetPolaris().Config(polaris.WithConfigFile(polaris.File{
		// 设置配置文件的名称
		Name: fileName,
		// 设置配置文件的组名
		Group: group,
	}))
}
