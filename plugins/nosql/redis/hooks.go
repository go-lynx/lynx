package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// metricsHook implements go-redis v9 Hook interface to record command-level metrics
type metricsHook struct{}

// DialHook: pass-through, we don't measure dial metrics here
func (metricsHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

// ProcessHook wraps single-command execution to observe latency and errors
func (metricsHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmd)
		redisCmdLatency.WithLabelValues(cmd.Name()).Observe(time.Since(start).Seconds())
		if err != nil && err != redis.Nil {
			redisCmdErrors.WithLabelValues(cmd.Name()).Inc()
		}
		return err
	}
}

// ProcessPipelineHook wraps pipeline execution to observe total latency and per-cmd errors
func (metricsHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmds)
		redisCmdLatency.WithLabelValues("pipeline").Observe(time.Since(start).Seconds())
		for _, c := range cmds {
			if e := c.Err(); e != nil && e != redis.Nil {
				redisCmdErrors.WithLabelValues(c.Name()).Inc()
			}
		}
		return err
	}
}
