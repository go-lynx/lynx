package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/config"
)

// GetConfig 方法用于从 Polaris 配置中心获取配置。
// 该方法会根据传入的配置文件名和配置文件组名，从 Polaris 配置中心获取对应的配置源。
// 参数 fileName 为要获取的配置文件的名称。
// 参数 group 为配置文件所在的组名。
// 返回值 config.Source 表示获取到的配置源，可用于后续的配置加载操作。
// 返回值 error 表示获取配置过程中可能出现的错误，若操作成功则为 nil。
func (p *PlugPolaris) GetConfig(fileName string, group string) (config.Source, error) {
	// 调用 GetPolaris() 函数获取 Polaris 实例，
	// 并使用该实例的 Config 方法结合 WithConfigFile 选项来设置要获取的配置文件信息。
	return GetPolaris().Config(
		// 使用 WithConfigFile 选项设置要获取的配置文件的详细信息
		polaris.WithConfigFile(
			polaris.File{
				// 设置要获取的配置文件的名称
				Name: fileName,
				// 设置配置文件所属的组名
				Group: group,
			}))
}
