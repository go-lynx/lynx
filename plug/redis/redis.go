package redis

import (
	"context"
	"github.com/go-lynx/lynx/boot"
	"github.com/go-lynx/lynx/conf"
	"github.com/go-lynx/lynx/plug"
	"github.com/redis/go-redis/v9"
	"time"
)

var plugName = "redis"

type PlugRedis struct {
	rdb *redis.Client
}

func (r *PlugRedis) Name() string {
	return plugName
}

func (r *PlugRedis) Weight() int {
	return 999
}

func (r *PlugRedis) Load(b *conf.Bootstrap) (plug.Plug, error) {
	boot.GetHelper().Infof("Initializing Redis")
	r.rdb = redis.NewClient(&redis.Options{
		Addr:         b.Data.Redis.Addr,
		Password:     b.Data.Redis.Password,
		DB:           int(b.Data.Redis.Db),
		DialTimeout:  b.Data.Redis.DialTimeout.AsDuration(),
		WriteTimeout: b.Data.Redis.WriteTimeout.AsDuration(),
		ReadTimeout:  b.Data.Redis.ReadTimeout.AsDuration(),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.rdb.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}
	boot.GetHelper().Infof("Redis successfully initialized")
	return r, nil
}

func (r *PlugRedis) Unload() error {
	boot.GetHelper().Info("message", "Closing the Redis resources")
	if err := r.rdb.Close(); err != nil {
		boot.GetHelper().Error(err)
		return err
	}
	return nil
}

func GetRedis() *redis.Client {
	return boot.GetPlug(plugName).(*PlugRedis).rdb
}

func Redis() plug.Plug {
	return &PlugRedis{}
}
