package mongodb

import (
	"time"

	"github.com/go-lynx/lynx/plugins/nosql/mongodb/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Option defines the plugin option function type
type Option func(*PlugMongoDB)

// WithURI sets the connection string
func WithURI(uri string) Option {
	return func(p *PlugMongoDB) {
		if p.conf == nil {
			p.conf = &conf.MongoDB{}
		}
		p.conf.Uri = uri
	}
}

// WithDatabase sets the database name
func WithDatabase(database string) Option {
	return func(p *PlugMongoDB) {
		if p.conf == nil {
			p.conf = &conf.MongoDB{}
		}
		p.conf.Database = database
	}
}

// WithCredentials sets authentication information
func WithCredentials(username, password, authSource string) Option {
	return func(p *PlugMongoDB) {
		if p.conf == nil {
			p.conf = &conf.MongoDB{}
		}
		p.conf.Username = username
		p.conf.Password = password
		p.conf.AuthSource = authSource
	}
}

// WithPoolSize sets connection pool size
func WithPoolSize(maxPoolSize, minPoolSize uint64) Option {
	return func(p *PlugMongoDB) {
		if p.conf == nil {
			p.conf = &conf.MongoDB{}
		}
		p.conf.MaxPoolSize = maxPoolSize
		p.conf.MinPoolSize = minPoolSize
	}
}

// WithTimeouts sets timeout configuration
func WithTimeouts(connectTimeout, serverSelectionTimeout, socketTimeout time.Duration) Option {
	return func(p *PlugMongoDB) {
		if p.conf == nil {
			p.conf = &conf.MongoDB{}
		}
		p.conf.ConnectTimeout = durationpb.New(connectTimeout)
		p.conf.ServerSelectionTimeout = durationpb.New(serverSelectionTimeout)
		p.conf.SocketTimeout = durationpb.New(socketTimeout)
	}
}

// WithHeartbeatInterval sets heartbeat interval
func WithHeartbeatInterval(interval time.Duration) Option {
	return func(p *PlugMongoDB) {
		if p.conf == nil {
			p.conf = &conf.MongoDB{}
		}
		p.conf.HeartbeatInterval = durationpb.New(interval)
	}
}

// WithMetrics sets metrics enablement
func WithMetrics(enable bool) Option {
	return func(p *PlugMongoDB) {
		if p.conf == nil {
			p.conf = &conf.MongoDB{}
		}
		p.conf.EnableMetrics = enable
	}
}

// WithHealthCheck sets health check configuration
func WithHealthCheck(enable bool, interval time.Duration) Option {
	return func(p *PlugMongoDB) {
		if p.conf == nil {
			p.conf = &conf.MongoDB{}
		}
		p.conf.EnableHealthCheck = enable
		p.conf.HealthCheckInterval = durationpb.New(interval)
	}
}

// WithTLS sets TLS configuration
func WithTLS(enable bool, certFile, keyFile, caFile string) Option {
	return func(p *PlugMongoDB) {
		if p.conf == nil {
			p.conf = &conf.MongoDB{}
		}
		p.conf.EnableTls = enable
		p.conf.TlsCertFile = certFile
		p.conf.TlsKeyFile = keyFile
		p.conf.TlsCaFile = caFile
	}
}

// WithCompression sets compression configuration
func WithCompression(enable bool, level int) Option {
	return func(p *PlugMongoDB) {
		if p.conf == nil {
			p.conf = &conf.MongoDB{}
		}
		p.conf.EnableCompression = enable
		p.conf.CompressionLevel = int32(level)
	}
}

// WithRetryWrites sets retry writes configuration
func WithRetryWrites(enable bool) Option {
	return func(p *PlugMongoDB) {
		if p.conf == nil {
			p.conf = &conf.MongoDB{}
		}
		p.conf.EnableRetryWrites = enable
	}
}

// WithReadConcern sets read concern configuration
func WithReadConcern(enable bool, level string) Option {
	return func(p *PlugMongoDB) {
		if p.conf == nil {
			p.conf = &conf.MongoDB{}
		}
		p.conf.EnableReadConcern = enable
		p.conf.ReadConcernLevel = level
	}
}

// WithWriteConcern sets write concern configuration
func WithWriteConcern(enable bool, w int, timeout time.Duration) Option {
	return func(p *PlugMongoDB) {
		if p.conf == nil {
			p.conf = &conf.MongoDB{}
		}
		p.conf.EnableWriteConcern = enable
		p.conf.WriteConcernW = int32(w)
		p.conf.WriteConcernTimeout = durationpb.New(timeout)
	}
}
