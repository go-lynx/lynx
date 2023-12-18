package tracer

import "github.com/go-kratos/kratos/v2/config"

func (t *PlugTracer) Weight() int {
	return t.weight
}

func (t *PlugTracer) DependsOn(config.Value) []string {
	return nil
}

func (t *PlugTracer) Name() string {
	return name
}

func (t *PlugTracer) ConfPrefix() string {
	return confPrefix
}
