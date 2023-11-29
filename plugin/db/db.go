package db

import (
	"context"
	_ "database/sql"
	"entgo.io/ent/dialect/sql"
	"fmt"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugin"
	"github.com/go-lynx/lynx/plugin/db/conf"
	"time"
)

var name = "db"

type PlugDB struct {
	dri    *sql.Driver
	weight int
}

type Option func(db *PlugDB)

func Weight(w int) Option {
	return func(db *PlugDB) {
		db.weight = w
	}
}

func (db *PlugDB) Name() string {
	return name
}

func (db *PlugDB) Weight() int {
	return db.weight
}

func (db *PlugDB) Load(base interface{}) (plugin.Plugin, error) {
	c, ok := base.(*conf.Db)
	if !ok {
		return nil, fmt.Errorf("invalid c type, expected *conf.Grpc")
	}

	app.Lynx().GetHelper().Infof("Initializing database")
	drv, err := sql.Open(
		c.Driver,
		c.Source,
	)

	if err != nil {
		app.Lynx().GetHelper().Errorf("failed opening connection to dataBase: %v", err)
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
	app.Lynx().GetHelper().Infof("Database successfully initialized")
	return db, nil
}

func (db *PlugDB) Unload() error {
	app.Lynx().GetHelper().Info("message", "Closing the DataBase resources")
	if err := db.dri.Close(); err != nil {
		app.Lynx().GetHelper().Error(err)
		return err
	}
	return nil
}

func Db(opts ...Option) plugin.Plugin {
	db := &PlugDB{
		weight: 1000,
	}
	for _, opt := range opts {
		opt(db)
	}
	return db
}
