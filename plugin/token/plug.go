package token

import (
	"github.com/go-lynx/lynx/factory"
	"github.com/go-lynx/lynx/plugin"
	"github.com/go-lynx/lynx/plugin/token/login"
)

func init() {
	factory.GlobalPluginFactory().Register(name, configPrefix, func() plugin.Plugin {
		return Token(login.NewLogin())
	})
}
