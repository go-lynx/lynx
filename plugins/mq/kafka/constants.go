package kafka

// Plugin metadata
const (
	pluginName        = "kafka.client"
	pluginVersion     = "v2.0.0"
	pluginDescription = "kafka client plugin for Lynx framework"
	confPrefix        = "lynx.kafka"
)

// Compression type constants
const (
	CompressionNone   = "none"
	CompressionGzip   = "gzip"
	CompressionSnappy = "snappy"
	CompressionLz4    = "lz4"
	CompressionZstd   = "zstd"
)

// SASL mechanism constants
const (
	SASLPlain       = "PLAIN"
	SASLScramSHA256 = "SCRAM-SHA-256"
	SASLScramSHA512 = "SCRAM-SHA-512"
)

// Consumer start offset constants
const (
	StartOffsetEarliest = "earliest"
	StartOffsetLatest   = "latest"
)
