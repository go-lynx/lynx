package db

import (
	"context"
	_ "database/sql"
	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugin"
	"github.com/go-lynx/lynx/plugin/db/conf"
	"time"
)

var (
	name       = "db"
	confPrefix = "lynx.db"
)

type PlugDB struct {
	dri    *sql.Driver
	conf   *conf.Db
	weight int
}

type Option func(db *PlugDB)

func Weight(w int) Option {
	return func(db *PlugDB) {
		db.weight = w
	}
}

func Config(c *conf.Db) Option {
	return func(db *PlugDB) {
		db.conf = c
	}
}

func (db *PlugDB) Name() string {
	return name
}

func (db *PlugDB) DependsOn() []string {
	return nil
}

func (db *PlugDB) Weight() int {
	return db.weight
}

func (db *PlugDB) ConfPrefix() string {
	return confPrefix
}

func (db *PlugDB) Load(b config.Value) (plugin.Plugin, error) {
	err := b.Scan(db.conf)
	if err != nil {
		return nil, err
	}

	app.Lynx().Helper().Infof("Initializing database")
	drv, err := sql.Open(
		db.conf.Driver,
		db.conf.Source,
	)

	if err != nil {
		app.Lynx().Helper().Errorf("failed opening connection to dataBase: %v", err)
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
	app.Lynx().Helper().Infof("Database successfully initialized")
	return db, nil
}

func (db *PlugDB) Unload() error {
	if db.dri == nil {
		return nil
	}
	if err := db.dri.Close(); err != nil {
		app.Lynx().Helper().Error(err)
		return err
	}
	app.Lynx().Helper().Info("message", "Closing the DataBase resources")
	return nil
}

func Db(opts ...Option) plugin.Plugin {
	db := &PlugDB{
		weight: 1000,
		conf:   &conf.Db{},
	}
	for _, opt := range opts {
		opt(db)
	}
	return db
}
