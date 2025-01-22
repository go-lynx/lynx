package redis

import "github.com/go-kratos/kratos/v2/config"

func (r *PlugRedis) Name() string {
	return name
}

func (r *PlugRedis) DependsOn(config.Value) []string {
	return nil
}

func (r *PlugRedis) ConfPrefix() string {
	return confPrefix
}

func (r *PlugRedis) Weight() int {
	return r.weight
}
