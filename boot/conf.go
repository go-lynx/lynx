package boot

import (
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/conf"
)

// localBootFileLoad Boot configuration file for service startup loaded from local
func (b *Boot) localBootFileLoad() *conf.Bootstrap {
	log.Info("Lynx reading local bootstrap configuration file/folder:" + flagConf)
	c := config.New(
		config.WithSource(
			file.NewSource(flagConf),
		),
	)
	if err := c.Load(); err != nil {
		panic(err)
	}
	var bootstrap conf.Bootstrap
	if err := c.Scan(&bootstrap); err != nil {
		panic(err)
	}
	// local file close
	defer func(c config.Config) {
		err := c.Close()
		if err != nil {
			panic(err)
		}
	}(c)
	b.conf, _ = c.Value("lynx").Map()
	return &bootstrap
}
