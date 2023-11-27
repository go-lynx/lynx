package rds

import (
	"context"
	_ "database/sql"
	"entgo.io/ent/dialect/sql"
	"fmt"
	"github.com/go-lynx/lynx/boot"
	"github.com/go-lynx/lynx/plugin"
	"github.com/go-lynx/lynx/plugin/rds/conf"
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

func (db *PlugMysql) Load(base interface{}) (plugin.Plugin, error) {
	c, ok := base.(*conf.Rds)
	if !ok {
		return nil, fmt.Errorf("invalid c type, expected *conf.Grpc")
	}

	boot.GetHelper().Infof("Initializing database")
	drv, err := sql.Open(
		c.Driver,
		c.Source,
	)

	if err != nil {
		boot.GetHelper().Errorf("failed opening connection to dataBase: %v", err)
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
