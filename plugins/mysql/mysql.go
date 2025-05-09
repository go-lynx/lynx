package mysql

import (
	"context"
	_ "database/sql"
	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/mysql/conf"
	"time"
)

// Plugin metadata
const (
	pluginName        = "mysql.client"
	pluginVersion     = "v2.0.0"
	pluginDescription = "Mysql client plugin for Lynx framework"
	confPrefix        = "lynx.mysql"
)

type PlugDB struct {
	*plugins.BasePlugin
	dri  *sql.Driver
	conf *conf.Mysql
}

type Option func(db *PlugDB)

// NewServiceHttp creates a new HTTP plugin instance
func NewMysqlHttp() *ServiceHttp {
	return &ServiceHttp{
		BasePlugin: plugins.NewBasePlugin(
			plugins.GeneratePluginID("", pluginName, pluginVersion),
			pluginName,
			pluginDescription,
			pluginVersion,
		),
	}
}

func Config(c *conf.Mysql) Option {
	return func(db *PlugDB) {
		db.conf = c
	}
}

func (db *PlugDB) Load(b config.Value) (plugins.Plugin, error) {
	err := b.Scan(db.conf)
	if err != nil {
		return nil, err
	}

	app.Lynx().GetLogHelper().Infof("Initializing database")
	drv, err := sql.Open(
		db.conf.Driver,
		db.conf.Source,
	)

	if err != nil {
		app.Lynx().GetLogHelper().Errorf("failed opening connection to dataBase: %v", err)
		panic(err)
	}

	drv.DB().SetMaxIdleConns(int(db.conf.MinConn))
	drv.DB().SetMaxOpenConns(int(db.conf.MaxConn))
	drv.DB().SetConnMaxIdleTime(db.conf.MaxIdleTime.AsDuration())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = drv.DB().PingContext(ctx)
	if err != nil {
		return nil, err
	}

	db.dri = drv
	app.Lynx().GetLogHelper().Infof("Database successfully initialized")
	return db, nil
}

func (db *PlugDB) Unload() error {
	if db.dri == nil {
		return nil
	}
	if err := db.dri.Close(); err != nil {
		app.Lynx().GetLogHelper().Error(err)
		return err
	}
	app.Lynx().GetLogHelper().Info("message", "Closing the DataBase resources")
	return nil
}
