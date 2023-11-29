package db

import (
	"entgo.io/ent/dialect/sql"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugin"
)

func init() {
	plugin.GlobalPluginFactory().Register(name, func() plugin.Plugin {
		return Db()
	})
}

func GetDriver() *sql.Driver {
	return app.Lynx().PlugManager().GetPlugin(name).(*PlugDB).dri
}
