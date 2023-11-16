package boot

import (
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-lynx/lynx/conf"
)

func configLoad(bc *conf.Bootstrap, s ...config.Source) {
	var c config.Config
	if s == nil {
		c = config.New(
			config.WithSource(
				file.NewSource(flagConf),
			),
		)
	} else {
		c = config.New(
			config.WithSource(
				s...,
			),
		)
	}

	if err := c.Load(); err != nil {
		panic(err)
	}

	if err := c.Scan(bc); err != nil {
		panic(err)
	}

	// local file close
	if s == nil {
		name = bc.Server.Name
		version = bc.Server.Version

		defer func(c config.Config) {
			err := c.Close()
			if err != nil {
				panic(err)
			}
		}(c)
	}
}
