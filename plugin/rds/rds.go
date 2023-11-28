package rds

import (
	"context"
	_ "database/sql"
	"entgo.io/ent/dialect/sql"
	"fmt"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/boot"
	"github.com/go-lynx/lynx/plugin"
	"github.com/go-lynx/lynx/plugin/rds/conf"
	"time"
)

var name = "mysql"

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
	return name
}

func (db *PlugMysql) Weight() int {
	return db.weight
}

func (db *PlugMysql) Load(base interface{}) (plugin.Plugin, error) {
	c, ok := base.(*conf.Rds)
	if !ok {
		return nil, fmt.Errorf("invalid c type, expected *conf.Grpc")
	}

	app.GetHelper().Infof("Initializing database")
	drv, err := sql.Open(
		c.Driver,
		c.Source,
	)

	if err != nil {
		app.GetHelper().Errorf("failed opening connection to dataBase: %v", err)
		panic(err)
	}

	drv.DB().SetMaxIdleConns(int(c.MinConn))
	drv.DB().SetMaxOpenConns(int(c.MaxConn))
	drv.DB().SetConnMaxIdleTime(c.MaxIdleTime.AsDuration())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = drv.DB().PingContext(ctx)
	if err != nil {
		return nil, err
	}

	db.dri = drv
	app.GetHelper().Infof("Database successfully initialized")
	return db, nil
}

func (db *PlugMysql) Unload() error {
	app.GetHelper().Info("message", "Closing the DataBase resources")
	if err := db.dri.Close(); err != nil {
		app.GetHelper().Error(err)
		return err
	}
	return nil
}

func GetDriver() *sql.Driver {
	return boot.GetPlugin(name).(*PlugMysql).dri
}

func Rds(opts ...Option) plugin.Plugin {
	db := &PlugMysql{
		weight: 1000,
	}
	for _, opt := range opts {
		opt(db)
	}
	return db
}
