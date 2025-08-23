package redis

import (
	"crypto/tls"
	"strings"

	"github.com/redis/go-redis/v9"
)

// buildUniversalOptions builds redis.UniversalOptions based on configuration
func (r *PlugRedis) buildUniversalOptions() *redis.UniversalOptions {
	// Parse addresses: prioritize addrs; fallback to addr if empty (supports comma separation)
	var addrs []string
	if len(r.conf.Addrs) > 0 {
		addrs = append(addrs, r.conf.Addrs...)
	}
	// TLS: configuration priority; then rediss:// inference
	var tlsConfig *tls.Config
	if r.conf.Tls != nil && r.conf.Tls.Enabled {
		tlsConfig = &tls.Config{InsecureSkipVerify: r.conf.Tls.InsecureSkipVerify}
	}
	for i := range addrs {
		if strings.HasPrefix(strings.ToLower(addrs[i]), "rediss://") {
			if tlsConfig == nil {
				tlsConfig = &tls.Config{}
			}
			addrs[i] = strings.TrimPrefix(addrs[i], "rediss://")
		}
	}
	// Sentinel: allow dedicated sentinel address override
	masterName := ""
	if r.conf.Sentinel != nil {
		masterName = r.conf.Sentinel.MasterName
		if len(r.conf.Sentinel.Addrs) > 0 {
			addrs = append([]string{}, r.conf.Sentinel.Addrs...)
		}
	}

	return &redis.UniversalOptions{
		Addrs:                 addrs,
		MasterName:            masterName,
		DB:                    int(r.conf.Db),
		Username:              r.conf.Username,
		Password:              r.conf.Password,
		MinIdleConns:          int(r.conf.MinIdleConns),
		PoolSize:              int(r.conf.MaxActiveConns),
		DialTimeout:           r.conf.DialTimeout.AsDuration(),
		ReadTimeout:           r.conf.ReadTimeout.AsDuration(),
		WriteTimeout:          r.conf.WriteTimeout.AsDuration(),
		ConnMaxIdleTime:       r.conf.ConnMaxIdleTime.AsDuration(),
		PoolTimeout:           r.conf.PoolTimeout.AsDuration(),
		MaxRetries:            int(r.conf.MaxRetries),
		MinRetryBackoff:       r.conf.MinRetryBackoff.AsDuration(),
		MaxRetryBackoff:       r.conf.MaxRetryBackoff.AsDuration(),
		ClientName:            r.conf.ClientName,
		TLSConfig:             tlsConfig,
		ContextTimeoutEnabled: true,
		ConnMaxLifetime:       r.conf.MaxConnAge.AsDuration(), // Map MaxConnAge to ConnMaxLifetime
	}
}
