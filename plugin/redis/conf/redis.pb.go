// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.32.0
// 	protoc        v4.23.0
// source: redis.proto

package conf

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Redis struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Network         string               `protobuf:"bytes,1,opt,name=network,proto3" json:"network,omitempty"`
	Addr            string               `protobuf:"bytes,2,opt,name=addr,proto3" json:"addr,omitempty"`
	Password        string               `protobuf:"bytes,3,opt,name=password,proto3" json:"password,omitempty"`
	Db              int32                `protobuf:"varint,4,opt,name=db,proto3" json:"db,omitempty"`
	MinIdleConns    int32                `protobuf:"varint,5,opt,name=min_idle_conns,json=minIdleConns,proto3" json:"min_idle_conns,omitempty"`
	MaxIdleConns    int32                `protobuf:"varint,6,opt,name=max_idle_conns,json=maxIdleConns,proto3" json:"max_idle_conns,omitempty"`
	MaxActiveConns  int32                `protobuf:"varint,7,opt,name=max_active_conns,json=maxActiveConns,proto3" json:"max_active_conns,omitempty"`
	ConnMaxIdleTime *durationpb.Duration `protobuf:"bytes,8,opt,name=conn_max_idle_time,json=connMaxIdleTime,proto3" json:"conn_max_idle_time,omitempty"`
	DialTimeout     *durationpb.Duration `protobuf:"bytes,9,opt,name=dial_timeout,json=dialTimeout,proto3" json:"dial_timeout,omitempty"`
	ReadTimeout     *durationpb.Duration `protobuf:"bytes,10,opt,name=read_timeout,json=readTimeout,proto3" json:"read_timeout,omitempty"`
	WriteTimeout    *durationpb.Duration `protobuf:"bytes,11,opt,name=write_timeout,json=writeTimeout,proto3" json:"write_timeout,omitempty"`
}

func (x *Redis) Reset() {
	*x = Redis{}
	if protoimpl.UnsafeEnabled {
		mi := &file_redis_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Redis) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Redis) ProtoMessage() {}

func (x *Redis) ProtoReflect() protoreflect.Message {
	mi := &file_redis_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Redis.ProtoReflect.Descriptor instead.
func (*Redis) Descriptor() ([]byte, []int) {
	return file_redis_proto_rawDescGZIP(), []int{0}
}

func (x *Redis) GetNetwork() string {
	if x != nil {
		return x.Network
	}
	return ""
}

func (x *Redis) GetAddr() string {
	if x != nil {
		return x.Addr
	}
	return ""
}

func (x *Redis) GetPassword() string {
	if x != nil {
		return x.Password
	}
	return ""
}

func (x *Redis) GetDb() int32 {
	if x != nil {
		return x.Db
	}
	return 0
}

func (x *Redis) GetMinIdleConns() int32 {
	if x != nil {
		return x.MinIdleConns
	}
	return 0
}

func (x *Redis) GetMaxIdleConns() int32 {
	if x != nil {
		return x.MaxIdleConns
	}
	return 0
}

func (x *Redis) GetMaxActiveConns() int32 {
	if x != nil {
		return x.MaxActiveConns
	}
	return 0
}

func (x *Redis) GetConnMaxIdleTime() *durationpb.Duration {
	if x != nil {
		return x.ConnMaxIdleTime
	}
	return nil
}

func (x *Redis) GetDialTimeout() *durationpb.Duration {
	if x != nil {
		return x.DialTimeout
	}
	return nil
}

func (x *Redis) GetReadTimeout() *durationpb.Duration {
	if x != nil {
		return x.ReadTimeout
	}
	return nil
}

func (x *Redis) GetWriteTimeout() *durationpb.Duration {
	if x != nil {
		return x.WriteTimeout
	}
	return nil
}

var File_redis_proto protoreflect.FileDescriptor

var file_redis_proto_rawDesc = []byte{
	0x0a, 0x0b, 0x72, 0x65, 0x64, 0x69, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x1a, 0x6c,
	0x79, 0x6e, 0x78, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x70, 0x6c, 0x75,
	0x67, 0x69, 0x6e, 0x2e, 0x72, 0x65, 0x64, 0x69, 0x73, 0x1a, 0x1e, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x64, 0x75, 0x72, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xdb, 0x03, 0x0a, 0x05, 0x72, 0x65,
	0x64, 0x69, 0x73, 0x12, 0x18, 0x0a, 0x07, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x12, 0x12, 0x0a,
	0x04, 0x61, 0x64, 0x64, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x61, 0x64, 0x64,
	0x72, 0x12, 0x1a, 0x0a, 0x08, 0x70, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x08, 0x70, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x12, 0x0e, 0x0a,
	0x02, 0x64, 0x62, 0x18, 0x04, 0x20, 0x01, 0x28, 0x05, 0x52, 0x02, 0x64, 0x62, 0x12, 0x24, 0x0a,
	0x0e, 0x6d, 0x69, 0x6e, 0x5f, 0x69, 0x64, 0x6c, 0x65, 0x5f, 0x63, 0x6f, 0x6e, 0x6e, 0x73, 0x18,
	0x05, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0c, 0x6d, 0x69, 0x6e, 0x49, 0x64, 0x6c, 0x65, 0x43, 0x6f,
	0x6e, 0x6e, 0x73, 0x12, 0x24, 0x0a, 0x0e, 0x6d, 0x61, 0x78, 0x5f, 0x69, 0x64, 0x6c, 0x65, 0x5f,
	0x63, 0x6f, 0x6e, 0x6e, 0x73, 0x18, 0x06, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0c, 0x6d, 0x61, 0x78,
	0x49, 0x64, 0x6c, 0x65, 0x43, 0x6f, 0x6e, 0x6e, 0x73, 0x12, 0x28, 0x0a, 0x10, 0x6d, 0x61, 0x78,
	0x5f, 0x61, 0x63, 0x74, 0x69, 0x76, 0x65, 0x5f, 0x63, 0x6f, 0x6e, 0x6e, 0x73, 0x18, 0x07, 0x20,
	0x01, 0x28, 0x05, 0x52, 0x0e, 0x6d, 0x61, 0x78, 0x41, 0x63, 0x74, 0x69, 0x76, 0x65, 0x43, 0x6f,
	0x6e, 0x6e, 0x73, 0x12, 0x46, 0x0a, 0x12, 0x63, 0x6f, 0x6e, 0x6e, 0x5f, 0x6d, 0x61, 0x78, 0x5f,
	0x69, 0x64, 0x6c, 0x65, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x08, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x19, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75,
	0x66, 0x2e, 0x44, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x0f, 0x63, 0x6f, 0x6e, 0x6e,
	0x4d, 0x61, 0x78, 0x49, 0x64, 0x6c, 0x65, 0x54, 0x69, 0x6d, 0x65, 0x12, 0x3c, 0x0a, 0x0c, 0x64,
	0x69, 0x61, 0x6c, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74, 0x18, 0x09, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x19, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x62, 0x75, 0x66, 0x2e, 0x44, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x0b, 0x64, 0x69,
	0x61, 0x6c, 0x54, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74, 0x12, 0x3c, 0x0a, 0x0c, 0x72, 0x65, 0x61,
	0x64, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x19, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75,
	0x66, 0x2e, 0x44, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x0b, 0x72, 0x65, 0x61, 0x64,
	0x54, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74, 0x12, 0x3e, 0x0a, 0x0d, 0x77, 0x72, 0x69, 0x74, 0x65,
	0x5f, 0x74, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74, 0x18, 0x0b, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19,
	0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2e, 0x44, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x0c, 0x77, 0x72, 0x69, 0x74, 0x65,
	0x54, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74, 0x42, 0x2b, 0x5a, 0x29, 0x67, 0x69, 0x74, 0x68, 0x75,
	0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x67, 0x6f, 0x2d, 0x6c, 0x79, 0x6e, 0x78, 0x2f, 0x6c, 0x79,
	0x6e, 0x78, 0x2f, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x2f, 0x72, 0x65, 0x64, 0x69, 0x73, 0x2f,
	0x63, 0x6f, 0x6e, 0x66, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_redis_proto_rawDescOnce sync.Once
	file_redis_proto_rawDescData = file_redis_proto_rawDesc
)

func file_redis_proto_rawDescGZIP() []byte {
	file_redis_proto_rawDescOnce.Do(func() {
		file_redis_proto_rawDescData = protoimpl.X.CompressGZIP(file_redis_proto_rawDescData)
	})
	return file_redis_proto_rawDescData
}

var file_redis_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_redis_proto_goTypes = []interface{}{
	(*Redis)(nil),               // 0: lynx.protobuf.plugin.redis.redis
	(*durationpb.Duration)(nil), // 1: google.protobuf.Duration
}
var file_redis_proto_depIdxs = []int32{
	1, // 0: lynx.protobuf.plugin.redis.redis.conn_max_idle_time:type_name -> google.protobuf.Duration
	1, // 1: lynx.protobuf.plugin.redis.redis.dial_timeout:type_name -> google.protobuf.Duration
	1, // 2: lynx.protobuf.plugin.redis.redis.read_timeout:type_name -> google.protobuf.Duration
	1, // 3: lynx.protobuf.plugin.redis.redis.write_timeout:type_name -> google.protobuf.Duration
	4, // [4:4] is the sub-list for method output_type
	4, // [4:4] is the sub-list for method input_type
	4, // [4:4] is the sub-list for extension type_name
	4, // [4:4] is the sub-list for extension extendee
	0, // [0:4] is the sub-list for field type_name
}

func init() { file_redis_proto_init() }
func file_redis_proto_init() {
	if File_redis_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_redis_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Redis); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_redis_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_redis_proto_goTypes,
		DependencyIndexes: file_redis_proto_depIdxs,
		MessageInfos:      file_redis_proto_msgTypes,
	}.Build()
	File_redis_proto = out.File
	file_redis_proto_rawDesc = nil
	file_redis_proto_goTypes = nil
	file_redis_proto_depIdxs = nil
}
