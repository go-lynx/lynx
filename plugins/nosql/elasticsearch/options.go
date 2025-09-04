package elasticsearch

import (
	"time"

	"github.com/go-lynx/lynx/plugins/nosql/elasticsearch/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Option defines the plugin option function type
type Option func(*PlugElasticsearch)

// WithAddresses sets the server addresses
func WithAddresses(addresses []string) Option {
	return func(p *PlugElasticsearch) {
		if p.conf == nil {
			p.conf = &conf.Elasticsearch{}
		}
		p.conf.Addresses = addresses
	}
}

// WithCredentials sets the authentication information
func WithCredentials(username, password string) Option {
	return func(p *PlugElasticsearch) {
		if p.conf == nil {
			p.conf = &conf.Elasticsearch{}
		}
		p.conf.Username = username
		p.conf.Password = password
	}
}

// WithAPIKey sets the API Key
func WithAPIKey(apiKey string) Option {
	return func(p *PlugElasticsearch) {
		if p.conf == nil {
			p.conf = &conf.Elasticsearch{}
		}
		p.conf.ApiKey = apiKey
	}
}

// WithServiceToken sets the service token
func WithServiceToken(token string) Option {
	return func(p *PlugElasticsearch) {
		if p.conf == nil {
			p.conf = &conf.Elasticsearch{}
		}
		p.conf.ServiceToken = token
	}
}

// WithCertificateFingerprint sets the certificate fingerprint
func WithCertificateFingerprint(fingerprint string) Option {
	return func(p *PlugElasticsearch) {
		if p.conf == nil {
			p.conf = &conf.Elasticsearch{}
		}
		p.conf.CertificateFingerprint = fingerprint
	}
}

// WithCompression sets request compression
func WithCompression(compress bool) Option {
	return func(p *PlugElasticsearch) {
		if p.conf == nil {
			p.conf = &conf.Elasticsearch{}
		}
		p.conf.CompressRequestBody = compress
	}
}

// WithConnectTimeout sets the connection timeout
func WithConnectTimeout(timeout time.Duration) Option {
	return func(p *PlugElasticsearch) {
		if p.conf == nil {
			p.conf = &conf.Elasticsearch{}
		}
		p.conf.ConnectTimeout = durationpb.New(timeout)
	}
}

// WithMaxRetries sets the maximum number of retries
func WithMaxRetries(maxRetries int) Option {
	return func(p *PlugElasticsearch) {
		if p.conf == nil {
			p.conf = &conf.Elasticsearch{}
		}
		p.conf.MaxRetries = int32(maxRetries)
	}
}

// WithMetrics sets metrics enablement
func WithMetrics(enable bool) Option {
	return func(p *PlugElasticsearch) {
		if p.conf == nil {
			p.conf = &conf.Elasticsearch{}
		}
		p.conf.EnableMetrics = enable
	}
}

// WithHealthCheck sets health check
func WithHealthCheck(enable bool, interval time.Duration) Option {
	return func(p *PlugElasticsearch) {
		if p.conf == nil {
			p.conf = &conf.Elasticsearch{}
		}
		p.conf.EnableHealthCheck = enable
		p.conf.HealthCheckInterval = durationpb.New(interval)
	}
}

// WithIndexPrefix sets the index prefix
func WithIndexPrefix(prefix string) Option {
	return func(p *PlugElasticsearch) {
		if p.conf == nil {
			p.conf = &conf.Elasticsearch{}
		}
		p.conf.IndexPrefix = prefix
	}
}

// WithLogLevel sets the log level
func WithLogLevel(level string) Option {
	return func(p *PlugElasticsearch) {
		if p.conf == nil {
			p.conf = &conf.Elasticsearch{}
		}
		p.conf.LogLevel = level
	}
}
