package boot

import (
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
)

// loadLocalBootFile Boot configuration file for service startup loaded from local
func (b *Boot) loadLocalBootFile() {
	// 打印日志，指示 Lynx 正在读取本地启动配置文件或文件夹
	log.Info("Lynx reading local bootstrap configuration file/folder:" + flagConf)

	// 创建一个新的配置对象，使用文件源加载配置
	c := config.New(
		config.WithSource(
			file.NewSource(flagConf),
		),
	)

	// 加载配置，如果加载过程中发生错误，抛出 panic
	if err := c.Load(); err != nil {
		panic(err)
	}

	// 延迟关闭本地文件，确保在函数结束时关闭文件
	defer func(c config.Config) {
		// 关闭配置对象，如果关闭过程中发生错误，抛出 panic
		err := c.Close()
		if err != nil {
			panic(err)
		}
	}(c)

	// 将加载的配置对象保存到 Boot 结构体的 conf 字段中
	b.conf = c
}
