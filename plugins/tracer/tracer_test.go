package tracer

import (
	"testing"

	"github.com/go-lynx/lynx/plugins/tracer/conf"
	"github.com/stretchr/testify/assert"
)

func TestPlugTracer_ValidateConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		config  *conf.Tracer
		wantErr bool
	}{
		{
			name: "valid configuration",
			config: &conf.Tracer{
				Enable: true,
				Addr:   "localhost:4317",
				Ratio:  0.5,
			},
			wantErr: false,
		},
		{
			name: "invalid ratio too high",
			config: &conf.Tracer{
				Enable: true,
				Addr:   "localhost:4317",
				Ratio:  1.5,
			},
			wantErr: true,
		},
		{
			name: "invalid ratio too low",
			config: &conf.Tracer{
				Enable: true,
				Addr:   "localhost:4317",
				Ratio:  -0.1,
			},
			wantErr: true,
		},
		{
			name: "enabled but no address",
			config: &conf.Tracer{
				Enable: true,
				Addr:   "",
				Ratio:  0.5,
			},
			wantErr: true,
		},
		{
			name: "disabled configuration",
			config: &conf.Tracer{
				Enable: false,
				Addr:   "",
				Ratio:  0.5,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracer := &PlugTracer{
				conf: tt.config,
			}
			err := tracer.validateConfiguration()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPlugTracer_SetDefaultValues(t *testing.T) {
	tests := []struct {
		name     string
		config   *conf.Tracer
		expected *conf.Tracer
	}{
		{
			name: "set default addr",
			config: &conf.Tracer{
				Enable: true,
				Addr:   "",
				Ratio:  0.5,
			},
			expected: &conf.Tracer{
				Enable: true,
				Addr:   "localhost:4317",
				Ratio:  0.5,
			},
		},
		{
			name: "set default ratio",
			config: &conf.Tracer{
				Enable: true,
				Addr:   "localhost:4317",
				Ratio:  0,
			},
			expected: &conf.Tracer{
				Enable: true,
				Addr:   "localhost:4317",
				Ratio:  1.0,
			},
		},
		{
			name: "no defaults needed",
			config: &conf.Tracer{
				Enable: true,
				Addr:   "localhost:4317",
				Ratio:  0.5,
			},
			expected: &conf.Tracer{
				Enable: true,
				Addr:   "localhost:4317",
				Ratio:  0.5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracer := &PlugTracer{
				conf: tt.config,
			}
			tracer.setDefaultValues()

			assert.Equal(t, tt.expected.Addr, tracer.conf.Addr)
			assert.Equal(t, tt.expected.Ratio, tracer.conf.Ratio)
		})
	}
}

func TestNewPlugTracer(t *testing.T) {
	tracer := NewPlugTracer()

	assert.NotNil(t, tracer)
	assert.NotNil(t, tracer.BasePlugin)
	assert.NotNil(t, tracer.conf)
}
