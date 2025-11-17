package conf

import (
	"google.golang.org/protobuf/types/known/durationpb"
)

// Default values for Nacos configuration
const (
	DefaultNamespace      = "public"
	DefaultGroup          = "DEFAULT_GROUP"
	DefaultCluster        = "DEFAULT"
	DefaultWeight         = 1.0
	DefaultTimeout        = 5  // seconds
	DefaultNotifyTimeout  = 3000  // milliseconds
	DefaultLogLevel       = "info"
	DefaultLogDir         = "./logs/nacos"
	DefaultCacheDir       = "./cache/nacos"
	DefaultContextPath    = "/nacos"
	DefaultHealthCheckInterval = 5  // seconds
	DefaultHealthCheckTimeout  = 3  // seconds
	DefaultHealthCheckType     = "tcp"
)

// GetDefaultTimeout returns default timeout duration
func GetDefaultTimeout() *durationpb.Duration {
	return durationpb.New(DefaultTimeout * 1000000000) // Convert to nanoseconds
}

