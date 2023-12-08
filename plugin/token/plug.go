package token

import (
	"github.com/go-lynx/lynx/plugin"
	"github.com/go-lynx/lynx/plugin/token/login"
)

func init() {
	plugin.GlobalPluginFactory().Register(name, configPrefix, func() plugin.Plugin {
		return Token(login.NewLogin())
	})
}
