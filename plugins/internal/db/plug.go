package db

import (
	"entgo.io/ent/dialect/sql"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/factory"
	"github.com/go-lynx/lynx/plugins"
)

func init() {
	factory.GlobalPluginFactory().RegisterPlugin(name, confPrefix, func() plugins.Plugin {
		return Db()
	})
}

func GetDriver() *sql.Driver {
	return app.Lynx().PlugManager().GetPlugin(name).(*PlugDB).dri
}
