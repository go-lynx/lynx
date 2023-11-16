package plug

import (
	"github.com/go-lynx/lynx/conf"
)

type Plug interface {
	// Weight 插件权重，权重越大越先加载
	Weight() int

	// Name 插件名称
	Name() string

	// Load 加载插件，给予配置文件进行对应插件加载
	Load(b *conf.Bootstrap) (Plug, error)

	// Unload 卸载插件
	Unload() error
}
