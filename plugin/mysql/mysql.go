package mysql

import (
	"context"
	_ "database/sql"
	"entgo.io/ent/dialect/sql"
	"github.com/go-lynx/lynx/boot"
	"github.com/go-lynx/lynx/conf"
	"github.com/go-lynx/lynx/plugin"
	"time"
)

var plugName = "mysql"

type PlugMysql struct {
	dri    *sql.Driver
	weight int
}

type Option func(db *PlugMysql)

func Weight(w int) Option {
	return func(db *PlugMysql) {
		db.weight = w
	}
}

func (db *PlugMysql) Name() string {
	return plugName
}

func (db *PlugMysql) Weight() int {
	return db.weight
}

func (db *PlugMysql) Load(b *conf.Bootstrap) (plugin.Plugin, error) {
	boot.GetHelper().Infof("Initializing database")
	drv, err := sql.Open(
		b.Data.Database.Driver,
		b.Data.Database.Source,
	)

	if err != nil {
		boot.GetHelper().Errorf("failed opening connection to dataBase: %v", err)
		panic(err)
	}

	drv.DB().SetMaxIdleConns(int(b.Data.Database.MinConn))
	drv.DB().SetMaxOpenConns(int(b.Data.Database.MaxConn))
	drv.DB().SetConnMaxIdleTime(b.Data.Database.MaxIdleTime.AsDuration())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = drv.DB().PingContext(ctx)
	if err != nil {
		return nil, err
	}

	db.dri = drv
	boot.GetHelper().Infof("Database successfully initialized")
	return db, nil
}

func (db *PlugMysql) Unload() error {
	boot.GetHelper().Info("message", "Closing the DataBase resources")
	if err := db.dri.Close(); err != nil {
		boot.GetHelper().Error(err)
		return err
	}
	return nil
}

func GetDB() *sql.Driver {
	return boot.GetPlugin(plugName).(*PlugMysql).dri
}

func Mysql(opts ...Option) plugin.Plugin {
	db := &PlugMysql{
		weight: 1000,
	}
	for _, opt := range opts {
		opt(db)
	}
	return db
}
