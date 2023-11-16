package mysql

import (
	"context"
	_ "database/sql"
	"entgo.io/ent/dialect/sql"
	"github.com/go-lynx/lynx/boot"
	"github.com/go-lynx/lynx/conf"
	"github.com/go-lynx/lynx/plug"
	"time"
)

var plugName = "mysql"

type PlugMysql struct {
	dri *sql.Driver
}

func (m *PlugMysql) Name() string {
	return plugName
}

func (m *PlugMysql) Weight() int {
	return 1000
}

func (m *PlugMysql) Load(b *conf.Bootstrap) (plug.Plug, error) {
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

	m.dri = drv
	boot.GetHelper().Infof("Database successfully initialized")
	return m, nil
}

func (m *PlugMysql) Unload() error {
	boot.GetHelper().Info("message", "Closing the DataBase resources")
	if err := m.dri.Close(); err != nil {
		boot.GetHelper().Error(err)
		return err
	}
	return nil
}

func GetDB() *sql.Driver {
	return boot.GetPlug(plugName).(*PlugMysql).dri
}

func Mysql() plug.Plug {
	return &PlugMysql{}
}
