// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        v4.23.0
// source: kafka.proto

package conf

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// Kafka 消息定义了 Kafka 客户端的配置信息
type Kafka struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// brokers 表示 Kafka 集群的地址列表
	Brokers []string `protobuf:"bytes,1,rep,name=brokers,proto3" json:"brokers,omitempty"`
	// producer 生产者配置
	Producer *Producer `protobuf:"bytes,2,opt,name=producer,proto3" json:"producer,omitempty"`
	// consumer 消费者配置
	Consumer *Consumer `protobuf:"bytes,3,opt,name=consumer,proto3" json:"consumer,omitempty"`
	// 通用配置
	// sasl 认证配置
	Sasl *SASL `protobuf:"bytes,4,opt,name=sasl,proto3" json:"sasl,omitempty"`
	// tls 配置
	Tls bool `protobuf:"varint,5,opt,name=tls,proto3" json:"tls,omitempty"`
	// 连接超时时间
	DialTimeout   *durationpb.Duration `protobuf:"bytes,6,opt,name=dial_timeout,json=dialTimeout,proto3" json:"dial_timeout,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Kafka) Reset() {
	*x = Kafka{}
	mi := &file_kafka_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Kafka) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Kafka) ProtoMessage() {}

func (x *Kafka) ProtoReflect() protoreflect.Message {
	mi := &file_kafka_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Kafka.ProtoReflect.Descriptor instead.
func (*Kafka) Descriptor() ([]byte, []int) {
	return file_kafka_proto_rawDescGZIP(), []int{0}
}

func (x *Kafka) GetBrokers() []string {
	if x != nil {
		return x.Brokers
	}
	return nil
}

func (x *Kafka) GetProducer() *Producer {
	if x != nil {
		return x.Producer
	}
	return nil
}

func (x *Kafka) GetConsumer() *Consumer {
	if x != nil {
		return x.Consumer
	}
	return nil
}

func (x *Kafka) GetSasl() *SASL {
	if x != nil {
		return x.Sasl
	}
	return nil
}

func (x *Kafka) GetTls() bool {
	if x != nil {
		return x.Tls
	}
	return false
}

func (x *Kafka) GetDialTimeout() *durationpb.Duration {
	if x != nil {
		return x.DialTimeout
	}
	return nil
}

// Producer 生产者配置
type Producer struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// 是否启用生产者
	Enabled bool `protobuf:"varint,1,opt,name=enabled,proto3" json:"enabled,omitempty"`
	// 是否需要等待所有副本确认
	RequiredAcks bool `protobuf:"varint,2,opt,name=required_acks,json=requiredAcks,proto3" json:"required_acks,omitempty"`
	// 最大重试次数
	MaxRetries int32 `protobuf:"varint,3,opt,name=max_retries,json=maxRetries,proto3" json:"max_retries,omitempty"`
	// 重试间隔
	RetryBackoff *durationpb.Duration `protobuf:"bytes,4,opt,name=retry_backoff,json=retryBackoff,proto3" json:"retry_backoff,omitempty"`
	// 批量发送大小
	BatchSize int32 `protobuf:"varint,5,opt,name=batch_size,json=batchSize,proto3" json:"batch_size,omitempty"`
	// 批量发送等待时间
	BatchTimeout *durationpb.Duration `protobuf:"bytes,6,opt,name=batch_timeout,json=batchTimeout,proto3" json:"batch_timeout,omitempty"`
	// 压缩类型：none, gzip, snappy, lz4, zstd
	Compression   string `protobuf:"bytes,7,opt,name=compression,proto3" json:"compression,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Producer) Reset() {
	*x = Producer{}
	mi := &file_kafka_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Producer) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Producer) ProtoMessage() {}

func (x *Producer) ProtoReflect() protoreflect.Message {
	mi := &file_kafka_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Producer.ProtoReflect.Descriptor instead.
func (*Producer) Descriptor() ([]byte, []int) {
	return file_kafka_proto_rawDescGZIP(), []int{1}
}

func (x *Producer) GetEnabled() bool {
	if x != nil {
		return x.Enabled
	}
	return false
}

func (x *Producer) GetRequiredAcks() bool {
	if x != nil {
		return x.RequiredAcks
	}
	return false
}

func (x *Producer) GetMaxRetries() int32 {
	if x != nil {
		return x.MaxRetries
	}
	return 0
}

func (x *Producer) GetRetryBackoff() *durationpb.Duration {
	if x != nil {
		return x.RetryBackoff
	}
	return nil
}

func (x *Producer) GetBatchSize() int32 {
	if x != nil {
		return x.BatchSize
	}
	return 0
}

func (x *Producer) GetBatchTimeout() *durationpb.Duration {
	if x != nil {
		return x.BatchTimeout
	}
	return nil
}

func (x *Producer) GetCompression() string {
	if x != nil {
		return x.Compression
	}
	return ""
}

// Consumer 消费者配置
type Consumer struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// 是否启用消费者
	Enabled bool `protobuf:"varint,1,opt,name=enabled,proto3" json:"enabled,omitempty"`
	// 消费组 ID
	GroupId string `protobuf:"bytes,2,opt,name=group_id,json=groupId,proto3" json:"group_id,omitempty"`
	// 自动提交间隔
	AutoCommitInterval *durationpb.Duration `protobuf:"bytes,3,opt,name=auto_commit_interval,json=autoCommitInterval,proto3" json:"auto_commit_interval,omitempty"`
	// 是否自动提交
	AutoCommit bool `protobuf:"varint,4,opt,name=auto_commit,json=autoCommit,proto3" json:"auto_commit,omitempty"`
	// 消费起始位置：latest, earliest
	StartOffset string `protobuf:"bytes,5,opt,name=start_offset,json=startOffset,proto3" json:"start_offset,omitempty"`
	// 最大处理并发数
	MaxConcurrency int32 `protobuf:"varint,6,opt,name=max_concurrency,json=maxConcurrency,proto3" json:"max_concurrency,omitempty"`
	// 最小批量大小
	MinBatchSize int32 `protobuf:"varint,7,opt,name=min_batch_size,json=minBatchSize,proto3" json:"min_batch_size,omitempty"`
	// 最大批量大小
	MaxBatchSize int32 `protobuf:"varint,8,opt,name=max_batch_size,json=maxBatchSize,proto3" json:"max_batch_size,omitempty"`
	// 最大等待时间
	MaxWaitTime *durationpb.Duration `protobuf:"bytes,9,opt,name=max_wait_time,json=maxWaitTime,proto3" json:"max_wait_time,omitempty"`
	// 重平衡超时时间
	RebalanceTimeout *durationpb.Duration `protobuf:"bytes,10,opt,name=rebalance_timeout,json=rebalanceTimeout,proto3" json:"rebalance_timeout,omitempty"`
	unknownFields    protoimpl.UnknownFields
	sizeCache        protoimpl.SizeCache
}

func (x *Consumer) Reset() {
	*x = Consumer{}
	mi := &file_kafka_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Consumer) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Consumer) ProtoMessage() {}

func (x *Consumer) ProtoReflect() protoreflect.Message {
	mi := &file_kafka_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Consumer.ProtoReflect.Descriptor instead.
func (*Consumer) Descriptor() ([]byte, []int) {
	return file_kafka_proto_rawDescGZIP(), []int{2}
}

func (x *Consumer) GetEnabled() bool {
	if x != nil {
		return x.Enabled
	}
	return false
}

func (x *Consumer) GetGroupId() string {
	if x != nil {
		return x.GroupId
	}
	return ""
}

func (x *Consumer) GetAutoCommitInterval() *durationpb.Duration {
	if x != nil {
		return x.AutoCommitInterval
	}
	return nil
}

func (x *Consumer) GetAutoCommit() bool {
	if x != nil {
		return x.AutoCommit
	}
	return false
}

func (x *Consumer) GetStartOffset() string {
	if x != nil {
		return x.StartOffset
	}
	return ""
}

func (x *Consumer) GetMaxConcurrency() int32 {
	if x != nil {
		return x.MaxConcurrency
	}
	return 0
}

func (x *Consumer) GetMinBatchSize() int32 {
	if x != nil {
		return x.MinBatchSize
	}
	return 0
}

func (x *Consumer) GetMaxBatchSize() int32 {
	if x != nil {
		return x.MaxBatchSize
	}
	return 0
}

func (x *Consumer) GetMaxWaitTime() *durationpb.Duration {
	if x != nil {
		return x.MaxWaitTime
	}
	return nil
}

func (x *Consumer) GetRebalanceTimeout() *durationpb.Duration {
	if x != nil {
		return x.RebalanceTimeout
	}
	return nil
}

// SASL 认证配置
type SASL struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// 是否启用 SASL
	Enabled bool `protobuf:"varint,1,opt,name=enabled,proto3" json:"enabled,omitempty"`
	// 认证机制：PLAIN, SCRAM-SHA-256, SCRAM-SHA-512
	Mechanism string `protobuf:"bytes,2,opt,name=mechanism,proto3" json:"mechanism,omitempty"`
	// 用户名
	Username string `protobuf:"bytes,3,opt,name=username,proto3" json:"username,omitempty"`
	// 密码
	Password      string `protobuf:"bytes,4,opt,name=password,proto3" json:"password,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SASL) Reset() {
	*x = SASL{}
	mi := &file_kafka_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SASL) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SASL) ProtoMessage() {}

func (x *SASL) ProtoReflect() protoreflect.Message {
	mi := &file_kafka_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SASL.ProtoReflect.Descriptor instead.
func (*SASL) Descriptor() ([]byte, []int) {
	return file_kafka_proto_rawDescGZIP(), []int{3}
}

func (x *SASL) GetEnabled() bool {
	if x != nil {
		return x.Enabled
	}
	return false
}

func (x *SASL) GetMechanism() string {
	if x != nil {
		return x.Mechanism
	}
	return ""
}

func (x *SASL) GetUsername() string {
	if x != nil {
		return x.Username
	}
	return ""
}

func (x *SASL) GetPassword() string {
	if x != nil {
		return x.Password
	}
	return ""
}

var File_kafka_proto protoreflect.FileDescriptor

const file_kafka_proto_rawDesc = "" +
	"\n" +
	"\vkafka.proto\x12\x1alynx.protobuf.plugin.kafka\x1a\x1egoogle/protobuf/duration.proto\"\xab\x02\n" +
	"\x05Kafka\x12\x18\n" +
	"\abrokers\x18\x01 \x03(\tR\abrokers\x12@\n" +
	"\bproducer\x18\x02 \x01(\v2$.lynx.protobuf.plugin.kafka.ProducerR\bproducer\x12@\n" +
	"\bconsumer\x18\x03 \x01(\v2$.lynx.protobuf.plugin.kafka.ConsumerR\bconsumer\x124\n" +
	"\x04sasl\x18\x04 \x01(\v2 .lynx.protobuf.plugin.kafka.SASLR\x04sasl\x12\x10\n" +
	"\x03tls\x18\x05 \x01(\bR\x03tls\x12<\n" +
	"\fdial_timeout\x18\x06 \x01(\v2\x19.google.protobuf.DurationR\vdialTimeout\"\xab\x02\n" +
	"\bProducer\x12\x18\n" +
	"\aenabled\x18\x01 \x01(\bR\aenabled\x12#\n" +
	"\rrequired_acks\x18\x02 \x01(\bR\frequiredAcks\x12\x1f\n" +
	"\vmax_retries\x18\x03 \x01(\x05R\n" +
	"maxRetries\x12>\n" +
	"\rretry_backoff\x18\x04 \x01(\v2\x19.google.protobuf.DurationR\fretryBackoff\x12\x1d\n" +
	"\n" +
	"batch_size\x18\x05 \x01(\x05R\tbatchSize\x12>\n" +
	"\rbatch_timeout\x18\x06 \x01(\v2\x19.google.protobuf.DurationR\fbatchTimeout\x12 \n" +
	"\vcompression\x18\a \x01(\tR\vcompression\"\xcc\x03\n" +
	"\bConsumer\x12\x18\n" +
	"\aenabled\x18\x01 \x01(\bR\aenabled\x12\x19\n" +
	"\bgroup_id\x18\x02 \x01(\tR\agroupId\x12K\n" +
	"\x14auto_commit_interval\x18\x03 \x01(\v2\x19.google.protobuf.DurationR\x12autoCommitInterval\x12\x1f\n" +
	"\vauto_commit\x18\x04 \x01(\bR\n" +
	"autoCommit\x12!\n" +
	"\fstart_offset\x18\x05 \x01(\tR\vstartOffset\x12'\n" +
	"\x0fmax_concurrency\x18\x06 \x01(\x05R\x0emaxConcurrency\x12$\n" +
	"\x0emin_batch_size\x18\a \x01(\x05R\fminBatchSize\x12$\n" +
	"\x0emax_batch_size\x18\b \x01(\x05R\fmaxBatchSize\x12=\n" +
	"\rmax_wait_time\x18\t \x01(\v2\x19.google.protobuf.DurationR\vmaxWaitTime\x12F\n" +
	"\x11rebalance_timeout\x18\n" +
	" \x01(\v2\x19.google.protobuf.DurationR\x10rebalanceTimeout\"v\n" +
	"\x04SASL\x12\x18\n" +
	"\aenabled\x18\x01 \x01(\bR\aenabled\x12\x1c\n" +
	"\tmechanism\x18\x02 \x01(\tR\tmechanism\x12\x1a\n" +
	"\busername\x18\x03 \x01(\tR\busername\x12\x1a\n" +
	"\bpassword\x18\x04 \x01(\tR\bpasswordB4Z2github.com/go-lynx/lynx/plugins/mq/kafka/conf;confb\x06proto3"

var (
	file_kafka_proto_rawDescOnce sync.Once
	file_kafka_proto_rawDescData []byte
)

func file_kafka_proto_rawDescGZIP() []byte {
	file_kafka_proto_rawDescOnce.Do(func() {
		file_kafka_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_kafka_proto_rawDesc), len(file_kafka_proto_rawDesc)))
	})
	return file_kafka_proto_rawDescData
}

var file_kafka_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_kafka_proto_goTypes = []any{
	(*Kafka)(nil),               // 0: lynx.protobuf.plugin.kafka.Kafka
	(*Producer)(nil),            // 1: lynx.protobuf.plugin.kafka.Producer
	(*Consumer)(nil),            // 2: lynx.protobuf.plugin.kafka.Consumer
	(*SASL)(nil),                // 3: lynx.protobuf.plugin.kafka.SASL
	(*durationpb.Duration)(nil), // 4: google.protobuf.Duration
}
var file_kafka_proto_depIdxs = []int32{
	1, // 0: lynx.protobuf.plugin.kafka.Kafka.producer:type_name -> lynx.protobuf.plugin.kafka.Producer
	2, // 1: lynx.protobuf.plugin.kafka.Kafka.consumer:type_name -> lynx.protobuf.plugin.kafka.Consumer
	3, // 2: lynx.protobuf.plugin.kafka.Kafka.sasl:type_name -> lynx.protobuf.plugin.kafka.SASL
	4, // 3: lynx.protobuf.plugin.kafka.Kafka.dial_timeout:type_name -> google.protobuf.Duration
	4, // 4: lynx.protobuf.plugin.kafka.Producer.retry_backoff:type_name -> google.protobuf.Duration
	4, // 5: lynx.protobuf.plugin.kafka.Producer.batch_timeout:type_name -> google.protobuf.Duration
	4, // 6: lynx.protobuf.plugin.kafka.Consumer.auto_commit_interval:type_name -> google.protobuf.Duration
	4, // 7: lynx.protobuf.plugin.kafka.Consumer.max_wait_time:type_name -> google.protobuf.Duration
	4, // 8: lynx.protobuf.plugin.kafka.Consumer.rebalance_timeout:type_name -> google.protobuf.Duration
	9, // [9:9] is the sub-list for method output_type
	9, // [9:9] is the sub-list for method input_type
	9, // [9:9] is the sub-list for extension type_name
	9, // [9:9] is the sub-list for extension extendee
	0, // [0:9] is the sub-list for field type_name
}

func init() { file_kafka_proto_init() }
func file_kafka_proto_init() {
	if File_kafka_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_kafka_proto_rawDesc), len(file_kafka_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_kafka_proto_goTypes,
		DependencyIndexes: file_kafka_proto_depIdxs,
		MessageInfos:      file_kafka_proto_msgTypes,
	}.Build()
	File_kafka_proto = out.File
	file_kafka_proto_goTypes = nil
	file_kafka_proto_depIdxs = nil
}
