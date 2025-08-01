// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        v4.23.0
// source: pgsql.proto

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

// Defines a message type for PgSQL configuration.
// 定义一个用于 PgSQL 数据库配置的消息类型。
type Pgsql struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// The driver name for the PgSQL database.
	// PgSQL 数据库的驱动名称。
	Driver string `protobuf:"bytes,1,opt,name=driver,proto3" json:"driver,omitempty"`
	// The data source name (DSN) for the PgSQL database.
	// PgSQL 数据库的数据源名称（DSN），用于连接数据库。
	Source string `protobuf:"bytes,2,opt,name=source,proto3" json:"source,omitempty"`
	// The minimum number of connections to maintain in the connection pool.
	// 连接池中需要维持的最小连接数。
	MinConn int32 `protobuf:"varint,3,opt,name=min_conn,json=minConn,proto3" json:"min_conn,omitempty"`
	// The maximum number of connections to maintain in the connection pool.
	// 连接池中需要维持的最大连接数。
	MaxConn int32 `protobuf:"varint,4,opt,name=max_conn,json=maxConn,proto3" json:"max_conn,omitempty"`
	// The maximum lifetime for a connection in the connection pool.
	// 连接池中连接的最大生命时间，超过该时间的连接可能会被关闭。
	MaxLifeTime *durationpb.Duration `protobuf:"bytes,5,opt,name=max_life_time,json=maxLifeTime,proto3" json:"max_life_time,omitempty"`
	// The maximum idle time for a connection in the connection pool.
	// 连接池中连接的最大空闲时间，超过该时间的连接可能会被关闭。
	MaxIdleTime   *durationpb.Duration `protobuf:"bytes,6,opt,name=max_idle_time,json=maxIdleTime,proto3" json:"max_idle_time,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Pgsql) Reset() {
	*x = Pgsql{}
	mi := &file_pgsql_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Pgsql) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Pgsql) ProtoMessage() {}

func (x *Pgsql) ProtoReflect() protoreflect.Message {
	mi := &file_pgsql_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Pgsql.ProtoReflect.Descriptor instead.
func (*Pgsql) Descriptor() ([]byte, []int) {
	return file_pgsql_proto_rawDescGZIP(), []int{0}
}

func (x *Pgsql) GetDriver() string {
	if x != nil {
		return x.Driver
	}
	return ""
}

func (x *Pgsql) GetSource() string {
	if x != nil {
		return x.Source
	}
	return ""
}

func (x *Pgsql) GetMinConn() int32 {
	if x != nil {
		return x.MinConn
	}
	return 0
}

func (x *Pgsql) GetMaxConn() int32 {
	if x != nil {
		return x.MaxConn
	}
	return 0
}

func (x *Pgsql) GetMaxLifeTime() *durationpb.Duration {
	if x != nil {
		return x.MaxLifeTime
	}
	return nil
}

func (x *Pgsql) GetMaxIdleTime() *durationpb.Duration {
	if x != nil {
		return x.MaxIdleTime
	}
	return nil
}

var File_pgsql_proto protoreflect.FileDescriptor

const file_pgsql_proto_rawDesc = "" +
	"\n" +
	"\vpgsql.proto\x12\x17lynx.protobuf.plugin.db\x1a\x1egoogle/protobuf/duration.proto\"\xeb\x01\n" +
	"\x05pgsql\x12\x16\n" +
	"\x06driver\x18\x01 \x01(\tR\x06driver\x12\x16\n" +
	"\x06source\x18\x02 \x01(\tR\x06source\x12\x19\n" +
	"\bmin_conn\x18\x03 \x01(\x05R\aminConn\x12\x19\n" +
	"\bmax_conn\x18\x04 \x01(\x05R\amaxConn\x12=\n" +
	"\rmax_life_time\x18\x05 \x01(\v2\x19.google.protobuf.DurationR\vmaxLifeTime\x12=\n" +
	"\rmax_idle_time\x18\x06 \x01(\v2\x19.google.protobuf.DurationR\vmaxIdleTimeB4Z2github.com/go-lynx/lynx/plugins/db/pgsql/conf;confb\x06proto3"

var (
	file_pgsql_proto_rawDescOnce sync.Once
	file_pgsql_proto_rawDescData []byte
)

func file_pgsql_proto_rawDescGZIP() []byte {
	file_pgsql_proto_rawDescOnce.Do(func() {
		file_pgsql_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_pgsql_proto_rawDesc), len(file_pgsql_proto_rawDesc)))
	})
	return file_pgsql_proto_rawDescData
}

var file_pgsql_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_pgsql_proto_goTypes = []any{
	(*Pgsql)(nil),               // 0: lynx.protobuf.plugin.db.pgsql
	(*durationpb.Duration)(nil), // 1: google.protobuf.Duration
}
var file_pgsql_proto_depIdxs = []int32{
	1, // 0: lynx.protobuf.plugin.db.pgsql.max_life_time:type_name -> google.protobuf.Duration
	1, // 1: lynx.protobuf.plugin.db.pgsql.max_idle_time:type_name -> google.protobuf.Duration
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_pgsql_proto_init() }
func file_pgsql_proto_init() {
	if File_pgsql_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_pgsql_proto_rawDesc), len(file_pgsql_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pgsql_proto_goTypes,
		DependencyIndexes: file_pgsql_proto_depIdxs,
		MessageInfos:      file_pgsql_proto_msgTypes,
	}.Build()
	File_pgsql_proto = out.File
	file_pgsql_proto_goTypes = nil
	file_pgsql_proto_depIdxs = nil
}
