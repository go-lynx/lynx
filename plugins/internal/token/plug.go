package token

import (
	"github.com/go-lynx/lynx/factory"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/internal/token/login"
)

func init() {
	factory.GlobalPluginFactory().Register(name, confPrefix, func() plugins.Plugin {
		return Token(login.NewLogin())
	})
}
