package redis

import (
	"context"
	"fmt"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugin"
	"github.com/go-lynx/lynx/plugin/redis/conf"
	"github.com/redis/go-redis/v9"
	"time"
)

var plugName = "redis"

type PlugRedis struct {
	rdb    *redis.Client
	weight int
}

type Option func(r *PlugRedis)

func Weight(w int) Option {
	return func(r *PlugRedis) {
		r.weight = w
	}
}

func (r *PlugRedis) Name() string {
	return plugName
}

func (r *PlugRedis) Weight() int {
	return r.weight
}

func (r *PlugRedis) Load(base interface{}) (plugin.Plugin, error) {
	c, ok := base.(*conf.Redis)
	if !ok {
		return nil, fmt.Errorf("invalid c type, expected *conf.Grpc")
	}

	app.Lynx().GetHelper().Infof("Initializing Redis")
	r.rdb = redis.NewClient(&redis.Options{
		Addr:         c.Addr,
		Password:     c.Password,
		DB:           int(c.Db),
		DialTimeout:  c.DialTimeout.AsDuration(),
		WriteTimeout: c.WriteTimeout.AsDuration(),
		ReadTimeout:  c.ReadTimeout.AsDuration(),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.rdb.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}
	app.Lynx().GetHelper().Infof("Redis successfully initialized")
	return r, nil
}

func (r *PlugRedis) Unload() error {
	app.Lynx().GetHelper().Info("message", "Closing the Redis resources")
	if err := r.rdb.Close(); err != nil {
		app.Lynx().GetHelper().Error(err)
		return err
	}
	return nil
}

func Redis(opts ...Option) plugin.Plugin {
	r := &PlugRedis{
		weight: 1001,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}
