// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.31.0
// 	protoc        v4.23.0
// source: tracer.proto

package conf

import (
	boot "github.com/go-lynx/lynx/boot"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Tracer struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Addr string     `protobuf:"bytes,1,opt,name=addr,proto3" json:"addr,omitempty"`
	Lynx *boot.Lynx `protobuf:"bytes,2,opt,name=lynx,proto3" json:"lynx,omitempty"`
}

func (x *Tracer) Reset() {
	*x = Tracer{}
	if protoimpl.UnsafeEnabled {
		mi := &file_tracer_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Tracer) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Tracer) ProtoMessage() {}

func (x *Tracer) ProtoReflect() protoreflect.Message {
	mi := &file_tracer_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Tracer.ProtoReflect.Descriptor instead.
func (*Tracer) Descriptor() ([]byte, []int) {
	return file_tracer_proto_rawDescGZIP(), []int{0}
}

func (x *Tracer) GetAddr() string {
	if x != nil {
		return x.Addr
	}
	return ""
}

func (x *Tracer) GetLynx() *boot.Lynx {
	if x != nil {
		return x.Lynx
	}
	return nil
}

var File_tracer_proto protoreflect.FileDescriptor

var file_tracer_proto_rawDesc = []byte{
	0x0a, 0x0c, 0x74, 0x72, 0x61, 0x63, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x14,
	0x6c, 0x79, 0x6e, 0x78, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x74, 0x72,
	0x61, 0x63, 0x65, 0x72, 0x1a, 0x0c, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x22, 0x45, 0x0a, 0x06, 0x54, 0x72, 0x61, 0x63, 0x65, 0x72, 0x12, 0x12, 0x0a, 0x04,
	0x61, 0x64, 0x64, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x61, 0x64, 0x64, 0x72,
	0x12, 0x27, 0x0a, 0x04, 0x6c, 0x79, 0x6e, 0x78, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x13,
	0x2e, 0x6c, 0x79, 0x6e, 0x78, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x4c,
	0x79, 0x6e, 0x78, 0x52, 0x04, 0x6c, 0x79, 0x6e, 0x78, 0x42, 0x2c, 0x5a, 0x2a, 0x67, 0x69, 0x74,
	0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x67, 0x6f, 0x2d, 0x6c, 0x79, 0x6e, 0x78, 0x2f,
	0x6c, 0x79, 0x6e, 0x78, 0x2f, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x2f, 0x74, 0x72, 0x61, 0x63,
	0x65, 0x72, 0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_tracer_proto_rawDescOnce sync.Once
	file_tracer_proto_rawDescData = file_tracer_proto_rawDesc
)

func file_tracer_proto_rawDescGZIP() []byte {
	file_tracer_proto_rawDescOnce.Do(func() {
		file_tracer_proto_rawDescData = protoimpl.X.CompressGZIP(file_tracer_proto_rawDescData)
	})
	return file_tracer_proto_rawDescData
}

var file_tracer_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_tracer_proto_goTypes = []interface{}{
	(*Tracer)(nil),    // 0: lynx.protobuf.tracer.Tracer
	(*boot.Lynx)(nil), // 1: lynx.protobuf.Lynx
}
var file_tracer_proto_depIdxs = []int32{
	1, // 0: lynx.protobuf.tracer.Tracer.lynx:type_name -> lynx.protobuf.Lynx
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_tracer_proto_init() }
func file_tracer_proto_init() {
	if File_tracer_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_tracer_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Tracer); i {
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
			RawDescriptor: file_tracer_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_tracer_proto_goTypes,
		DependencyIndexes: file_tracer_proto_depIdxs,
		MessageInfos:      file_tracer_proto_msgTypes,
	}.Build()
	File_tracer_proto = out.File
	file_tracer_proto_rawDesc = nil
	file_tracer_proto_goTypes = nil
	file_tracer_proto_depIdxs = nil
}