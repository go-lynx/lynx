package boot

import (
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
)

func configLoad(lynx *Lynx, s ...config.Source) {
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

	if err := c.Scan(lynx); err != nil {
		panic(err)
	}

	// local file close
	if s == nil {
		name = lynx.Application.Name
		version = lynx.Application.Version

		defer func(c config.Config) {
			err := c.Close()
			if err != nil {
				panic(err)
			}
		}(c)
	}
}
