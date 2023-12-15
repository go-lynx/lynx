package redis

import (
	"context"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugin"
	"github.com/go-lynx/lynx/plugin/redis/conf"
	"github.com/redis/go-redis/v9"
	"time"
)

var (
	name       = "redis"
	confPrefix = "lynx.redis"
)

type PlugRedis struct {
	rdb    *redis.Client
	conf   *conf.Redis
	weight int
}

type Option func(r *PlugRedis)

func Weight(w int) Option {
	return func(r *PlugRedis) {
		r.weight = w
	}
}

func Config(c *conf.Redis) Option {
	return func(r *PlugRedis) {
		r.conf = c
	}
}

func (r *PlugRedis) Name() string {
	return name
}

func (r *PlugRedis) DependsOn() []string {
	return nil
}

func (r *PlugRedis) ConfPrefix() string {
	return confPrefix
}

func (r *PlugRedis) Weight() int {
	return r.weight
}

func (r *PlugRedis) Load(b config.Value) (plugin.Plugin, error) {
	err := b.Scan(r.conf)
	if err != nil {
		return nil, err
	}

	app.Lynx().Helper().Infof("Initializing Redis")
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
	_, err = r.rdb.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}
	app.Lynx().Helper().Infof("Redis successfully initialized")
	return r, nil
}

func (r *PlugRedis) Unload() error {
	if r.rdb == nil {
		return nil
	}
	if err := r.rdb.Close(); err != nil {
		app.Lynx().Helper().Error(err)
		return err
	}
	app.Lynx().Helper().Info("message", "Closing the Redis resources")
	return nil
}

func Redis(opts ...Option) plugin.Plugin {
	r := &PlugRedis{
		weight: 1001,
		conf:   &conf.Redis{},
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}
