package base

import (
	"context"
	"database/sql"
	"time"

	"entgo.io/ent/dialect"
	esql "entgo.io/ent/dialect/sql"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"google.golang.org/protobuf/types/known/durationpb"
)

type Config interface {
	GetDriver() string
	GetSource() string
	GetMinConn() int32
	GetMaxConn() int32
	GetMaxIdleTime() *durationpb.Duration
	GetMaxLifeTime() *durationpb.Duration
}

// ConnectionPoolStats represents database connection pool statistics
type ConnectionPoolStats struct {
	MaxOpenConnections int
	OpenConnections    int
	InUse              int
	Idle               int
	MaxIdleConnections int
	WaitCount          int64
	WaitDuration       time.Duration
	MaxIdleClosed      int64
	MaxLifetimeClosed  int64
}

type SQLPlugin struct {
	*plugins.BasePlugin
	DB         *sql.DB
	drv        *esql.Driver
	conf       Config
	confPrefix string
	stats      *ConnectionPoolStats
	closed     bool
}

func NewSQLPlugin(id, name, desc, ver, confPrefix string, weight int, conf Config) *SQLPlugin {
	return &SQLPlugin{
		BasePlugin: plugins.NewBasePlugin(id, name, desc, ver, confPrefix, weight),
		conf:       conf,
		confPrefix: confPrefix,
		stats:      &ConnectionPoolStats{},
		closed:     false,
	}
}

func (p *SQLPlugin) InitializeResources(rt plugins.Runtime) error {
	if err := rt.GetConfig().Value(p.confPrefix).Scan(p.conf); err != nil {
		return err
	}
	return nil
}

func (p *SQLPlugin) StartupTasks() error {
	log.Infof("Initializing database: %v", p.Name())

	drv, err := esql.Open(p.conf.GetDriver(), p.conf.GetSource())
	if err != nil {
		log.Errorf("failed opening connection to dataBase: %v", err)
		return err
	}

	p.drv = drv
	p.DB = drv.DB()

	p.DB.SetMaxIdleConns(int(p.conf.GetMinConn()))
	p.DB.SetMaxOpenConns(int(p.conf.GetMaxConn()))
	p.DB.SetConnMaxIdleTime(p.conf.GetMaxIdleTime().AsDuration())
	p.DB.SetConnMaxLifetime(p.conf.GetMaxLifeTime().AsDuration())

	log.Infof("database successfully initialized: %v", p.Name())
	return nil
}

func (p *SQLPlugin) CleanupTasks() error {
	if p.drv == nil {
		return nil
	}
	if err := p.drv.Close(); err != nil {
		log.Error(err)
		return err
	}
	log.Info("message", "Closing the DataBase resources for: "+p.Name())
	return nil
}

func (p *SQLPlugin) CheckHealth() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.DB.PingContext(ctx); err != nil {
		return err
	}
	return nil
}

func (p *SQLPlugin) GetDriver() dialect.Driver {
	return p.drv
}

// GetStats returns connection pool statistics
func (p *SQLPlugin) GetStats() *ConnectionPoolStats {
	p.updateStats()
	return p.stats
}

// updateStats updates connection pool statistics from database
func (p *SQLPlugin) updateStats() {
	if p.DB == nil {
		return
	}
	dbStats := p.DB.Stats()
	p.stats.MaxOpenConnections = dbStats.MaxOpenConnections
	p.stats.OpenConnections = dbStats.OpenConnections
	p.stats.InUse = dbStats.InUse
	p.stats.Idle = dbStats.Idle
	p.stats.MaxIdleConnections = int(p.conf.GetMinConn())
	p.stats.WaitCount = dbStats.WaitCount
	p.stats.WaitDuration = dbStats.WaitDuration
	p.stats.MaxIdleClosed = dbStats.MaxIdleClosed
	p.stats.MaxLifetimeClosed = dbStats.MaxLifetimeClosed
}

// IsConnected checks if database is connected
func (p *SQLPlugin) IsConnected() bool {
	return !p.closed && p.DB != nil
}

// GetConfig returns current configuration
func (p *SQLPlugin) GetConfig() Config {
	return p.conf
}
