package mysql

import (
	"entgo.io/ent/dialect/sql"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/factory"
	"github.com/go-lynx/lynx/plugins"
)

func init() {
	factory.GlobalPluginFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		return Db()
	})
}

func GetDriver() *sql.Driver {
	return app.Lynx().GetPluginManager().GetPlugin(pluginName).(*PlugDB).dri
}
