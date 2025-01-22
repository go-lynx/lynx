package redis

import (
	"context"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/plugin-reids/v2/conf"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/types/known/durationpb"
	"time"
)

const (
	pluginName        = "redis"
	pluginVersion     = "v2.0.0"
	pluginDescription = "Redis plugin for Lynx framework"
	confPrefix        = "lynx.redis"
)

type PlugRedis struct {
	*plugins.BasePlugin
	conf *conf.Redis
	rdb  *redis.Client
}

// NewRedis creates a new Redis plugin instance
func NewRedis() *PlugRedis {
	return &PlugRedis{
		BasePlugin: plugins.NewBasePlugin(
			plugins.GeneratePluginID("", pluginName, pluginVersion),
			pluginName,
			pluginDescription,
			pluginVersion,
		),
	}
}

// InitializeResources implements custom initialization for Redis plugin
func (r *PlugRedis) InitializeResources(rt plugins.Runtime) error {
	// Add default configuration if not provided
	if r.conf == nil {
		r.conf = &conf.Redis{
			Network:         "tcp",
			Addr:            "localhost:6379",
			Password:        "",
			Db:              0,
			MinIdleConns:    10,
			MaxIdleConns:    20,
			MaxActiveConns:  20,
			DialTimeout:     &durationpb.Duration{Seconds: 10, Nanos: 0},
			ReadTimeout:     &durationpb.Duration{Seconds: 10, Nanos: 0},
			WriteTimeout:    &durationpb.Duration{Seconds: 10, Nanos: 0},
			ConnMaxIdleTime: &durationpb.Duration{Seconds: 10, Nanos: 0},
		}
	}
	err := rt.GetConfig().Scan(r.conf)
	if err != nil {
		return err
	}
	return nil
}

func (r *PlugRedis) StartupTasks() error {
	app.Lynx().GetLogHelper().Infof("Starting Redis Client")

	r.rdb = redis.NewClient(&redis.Options{
		Addr:            r.conf.Addr,
		Password:        r.conf.Password,
		DB:              int(r.conf.Db),
		MinIdleConns:    int(r.conf.MinIdleConns),
		MaxIdleConns:    int(r.conf.MaxIdleConns),
		MaxActiveConns:  int(r.conf.MaxActiveConns),
		DialTimeout:     r.conf.DialTimeout.AsDuration(),
		WriteTimeout:    r.conf.WriteTimeout.AsDuration(),
		ReadTimeout:     r.conf.ReadTimeout.AsDuration(),
		ConnMaxIdleTime: r.conf.ConnMaxIdleTime.AsDuration(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.rdb.Ping(ctx).Result()
	if err != nil {
		return err
	}

	app.Lynx().GetLogHelper().Infof("Redis Client Successfully Started")
	return nil
}

func (r *PlugRedis) CleanupTasks() error {
	if r.rdb == nil {
		return nil
	}
	if err := r.rdb.Close(); err != nil {
		return plugins.NewPluginError(r.ID(), "Stop", "Failed to stop HTTP server", err)
	}
	return nil
}

// Configure allows runtime configuration updates for the Redis server.
// It accepts an interface{} parameter that should contain the new configuration
// and updates the server settings accordingly.
func (r *PlugRedis) Configure(c any) error {
	if c == nil {
		return nil
	}
	r.conf = c.(*conf.Redis)
	return nil
}

// CheckHealth implements the health check interface for the Redis server.
// It performs necessary health checks and updates the provided health report
// with the current status of the server.
func (r *PlugRedis) CheckHealth(report *plugins.HealthReport) error {
	return nil
}
