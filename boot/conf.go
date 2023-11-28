package boot

import (
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
)

// localBootFileLoad Boot configuration file for service startup loaded from local
func localBootFileLoad() *Lynx {
	c := config.New(
		config.WithSource(
			file.NewSource(flagConf),
		),
	)
	if err := c.Load(); err != nil {
		panic(err)
	}
	var lynx Lynx
	if err := c.Scan(&lynx); err != nil {
		panic(err)
	}

	// local file close
	defer func(c config.Config) {
		err := c.Close()
		if err != nil {
			panic(err)
		}
	}(c)
	return &lynx
}
