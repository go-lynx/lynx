package kafka

// 插件元数据
const (
	pluginName        = "kafka.client"
	pluginVersion     = "v2.0.0"
	pluginDescription = "kafka client plugin for Lynx framework"
	confPrefix        = "lynx.kafka"
)

// 压缩类型常量
const (
	CompressionNone   = "none"
	CompressionGzip   = "gzip"
	CompressionSnappy = "snappy"
	CompressionLz4    = "lz4"
	CompressionZstd   = "zstd"
)

// SASL 机制常量
const (
	SASLPlain       = "PLAIN"
	SASLScramSHA256 = "SCRAM-SHA-256"
	SASLScramSHA512 = "SCRAM-SHA-512"
)

// 消费起始位置常量
const (
	StartOffsetEarliest = "earliest"
	StartOffsetLatest   = "latest"
)
