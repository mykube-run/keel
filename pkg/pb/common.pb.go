// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.15.5
// source: common.proto

package pb

import (
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/options"
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

type Code int32

const (
	Code_Ok                       Code = 0
	Code_PermissionDenied         Code = 1
	Code_InvalidRequestParameters Code = 2
	Code_ResourceAlreadyExists    Code = 3
	Code_TenantNotExist           Code = 4
	Code_TaskNotExist             Code = 5
	Code_TenantQuotaExceeded      Code = 6
	Code_TaskFinished             Code = 7
	Code_TaskNotRunning           Code = 8
)

// Enum value maps for Code.
var (
	Code_name = map[int32]string{
		0: "Ok",
		1: "PermissionDenied",
		2: "InvalidRequestParameters",
		3: "ResourceAlreadyExists",
		4: "TenantNotExist",
		5: "TaskNotExist",
		6: "TenantQuotaExceeded",
		7: "TaskFinished",
		8: "TaskNotRunning",
	}
	Code_value = map[string]int32{
		"Ok":                       0,
		"PermissionDenied":         1,
		"InvalidRequestParameters": 2,
		"ResourceAlreadyExists":    3,
		"TenantNotExist":           4,
		"TaskNotExist":             5,
		"TenantQuotaExceeded":      6,
		"TaskFinished":             7,
		"TaskNotRunning":           8,
	}
)

func (x Code) Enum() *Code {
	p := new(Code)
	*p = x
	return p
}

func (x Code) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Code) Descriptor() protoreflect.EnumDescriptor {
	return file_common_proto_enumTypes[0].Descriptor()
}

func (Code) Type() protoreflect.EnumType {
	return &file_common_proto_enumTypes[0]
}

func (x Code) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Code.Descriptor instead.
func (Code) EnumDescriptor() ([]byte, []int) {
	return file_common_proto_rawDescGZIP(), []int{0}
}

type Response struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Code    Code   `protobuf:"varint,1,opt,name=code,proto3,enum=pb.Code" json:"code,omitempty"`
	Message string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
}

func (x *Response) Reset() {
	*x = Response{}
	if protoimpl.UnsafeEnabled {
		mi := &file_common_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Response) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Response) ProtoMessage() {}

func (x *Response) ProtoReflect() protoreflect.Message {
	mi := &file_common_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Response.ProtoReflect.Descriptor instead.
func (*Response) Descriptor() ([]byte, []int) {
	return file_common_proto_rawDescGZIP(), []int{0}
}

func (x *Response) GetCode() Code {
	if x != nil {
		return x.Code
	}
	return Code_Ok
}

func (x *Response) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

type ResourceQuota struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Concurrency int64   `protobuf:"varint,1,opt,name=concurrency,proto3" json:"concurrency,omitempty"`
	Cpu         int64   `protobuf:"varint,2,opt,name=cpu,proto3" json:"cpu,omitempty"`
	Custom      int64   `protobuf:"varint,3,opt,name=custom,proto3" json:"custom,omitempty"`
	Gpu         int64   `protobuf:"varint,4,opt,name=gpu,proto3" json:"gpu,omitempty"`
	Memory      int64   `protobuf:"varint,5,opt,name=memory,proto3" json:"memory,omitempty"`
	Storage     int64   `protobuf:"varint,6,opt,name=storage,proto3" json:"storage,omitempty"`
	Peak        float32 `protobuf:"fixed32,7,opt,name=peak,proto3" json:"peak,omitempty"`
}

func (x *ResourceQuota) Reset() {
	*x = ResourceQuota{}
	if protoimpl.UnsafeEnabled {
		mi := &file_common_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ResourceQuota) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ResourceQuota) ProtoMessage() {}

func (x *ResourceQuota) ProtoReflect() protoreflect.Message {
	mi := &file_common_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ResourceQuota.ProtoReflect.Descriptor instead.
func (*ResourceQuota) Descriptor() ([]byte, []int) {
	return file_common_proto_rawDescGZIP(), []int{1}
}

func (x *ResourceQuota) GetConcurrency() int64 {
	if x != nil {
		return x.Concurrency
	}
	return 0
}

func (x *ResourceQuota) GetCpu() int64 {
	if x != nil {
		return x.Cpu
	}
	return 0
}

func (x *ResourceQuota) GetCustom() int64 {
	if x != nil {
		return x.Custom
	}
	return 0
}

func (x *ResourceQuota) GetGpu() int64 {
	if x != nil {
		return x.Gpu
	}
	return 0
}

func (x *ResourceQuota) GetMemory() int64 {
	if x != nil {
		return x.Memory
	}
	return 0
}

func (x *ResourceQuota) GetStorage() int64 {
	if x != nil {
		return x.Storage
	}
	return 0
}

func (x *ResourceQuota) GetPeak() float32 {
	if x != nil {
		return x.Peak
	}
	return 0
}

var File_common_proto protoreflect.FileDescriptor

var file_common_proto_rawDesc = []byte{
	0x0a, 0x0c, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x02,
	0x70, 0x62, 0x1a, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x2d, 0x67, 0x65, 0x6e, 0x2d, 0x6f,
	0x70, 0x65, 0x6e, 0x61, 0x70, 0x69, 0x76, 0x32, 0x2f, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73,
	0x2f, 0x61, 0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x22, 0x6d, 0x0a, 0x08, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x30,
	0x0a, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x08, 0x2e, 0x70,
	0x62, 0x2e, 0x43, 0x6f, 0x64, 0x65, 0x42, 0x12, 0x92, 0x41, 0x0f, 0x32, 0x0d, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x20, 0x63, 0x6f, 0x64, 0x65, 0x52, 0x04, 0x63, 0x6f, 0x64, 0x65,
	0x12, 0x2f, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x42, 0x15, 0x92, 0x41, 0x12, 0x32, 0x10, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x20, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67,
	0x65, 0x22, 0x97, 0x03, 0x0a, 0x0d, 0x52, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x51, 0x75,
	0x6f, 0x74, 0x61, 0x12, 0x3d, 0x0a, 0x0b, 0x63, 0x6f, 0x6e, 0x63, 0x75, 0x72, 0x72, 0x65, 0x6e,
	0x63, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x42, 0x1b, 0x92, 0x41, 0x18, 0x32, 0x16, 0x54,
	0x61, 0x73, 0x6b, 0x20, 0x63, 0x6f, 0x6e, 0x63, 0x75, 0x72, 0x72, 0x65, 0x6e, 0x63, 0x79, 0x20,
	0x71, 0x75, 0x6f, 0x74, 0x61, 0x52, 0x0b, 0x63, 0x6f, 0x6e, 0x63, 0x75, 0x72, 0x72, 0x65, 0x6e,
	0x63, 0x79, 0x12, 0x29, 0x0a, 0x03, 0x63, 0x70, 0x75, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x42,
	0x17, 0x92, 0x41, 0x14, 0x32, 0x12, 0x43, 0x50, 0x55, 0x20, 0x71, 0x75, 0x6f, 0x74, 0x61, 0x20,
	0x69, 0x6e, 0x20, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x52, 0x03, 0x63, 0x70, 0x75, 0x12, 0x32, 0x0a,
	0x06, 0x63, 0x75, 0x73, 0x74, 0x6f, 0x6d, 0x18, 0x03, 0x20, 0x01, 0x28, 0x03, 0x42, 0x1a, 0x92,
	0x41, 0x17, 0x32, 0x15, 0x43, 0x75, 0x73, 0x74, 0x6f, 0x6d, 0x20, 0x72, 0x65, 0x73, 0x6f, 0x75,
	0x72, 0x63, 0x65, 0x20, 0x71, 0x75, 0x6f, 0x74, 0x61, 0x52, 0x06, 0x63, 0x75, 0x73, 0x74, 0x6f,
	0x6d, 0x12, 0x29, 0x0a, 0x03, 0x67, 0x70, 0x75, 0x18, 0x04, 0x20, 0x01, 0x28, 0x03, 0x42, 0x17,
	0x92, 0x41, 0x14, 0x32, 0x12, 0x47, 0x50, 0x55, 0x20, 0x71, 0x75, 0x6f, 0x74, 0x61, 0x20, 0x69,
	0x6e, 0x20, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x52, 0x03, 0x67, 0x70, 0x75, 0x12, 0x2f, 0x0a, 0x06,
	0x6d, 0x65, 0x6d, 0x6f, 0x72, 0x79, 0x18, 0x05, 0x20, 0x01, 0x28, 0x03, 0x42, 0x17, 0x92, 0x41,
	0x14, 0x32, 0x12, 0x4d, 0x65, 0x6d, 0x6f, 0x72, 0x79, 0x20, 0x71, 0x75, 0x6f, 0x74, 0x61, 0x20,
	0x69, 0x6e, 0x20, 0x4d, 0x42, 0x52, 0x06, 0x6d, 0x65, 0x6d, 0x6f, 0x72, 0x79, 0x12, 0x32, 0x0a,
	0x07, 0x73, 0x74, 0x6f, 0x72, 0x61, 0x67, 0x65, 0x18, 0x06, 0x20, 0x01, 0x28, 0x03, 0x42, 0x18,
	0x92, 0x41, 0x15, 0x32, 0x13, 0x53, 0x74, 0x6f, 0x72, 0x61, 0x67, 0x65, 0x20, 0x71, 0x75, 0x6f,
	0x74, 0x61, 0x20, 0x69, 0x6e, 0x20, 0x4d, 0x42, 0x52, 0x07, 0x73, 0x74, 0x6f, 0x72, 0x61, 0x67,
	0x65, 0x12, 0x58, 0x0a, 0x04, 0x70, 0x65, 0x61, 0x6b, 0x18, 0x07, 0x20, 0x01, 0x28, 0x02, 0x42,
	0x44, 0x92, 0x41, 0x41, 0x32, 0x3f, 0x4d, 0x61, 0x78, 0x69, 0x6d, 0x75, 0x6d, 0x20, 0x72, 0x61,
	0x74, 0x69, 0x6f, 0x20, 0x6f, 0x66, 0x20, 0x74, 0x68, 0x65, 0x73, 0x65, 0x20, 0x71, 0x75, 0x6f,
	0x74, 0x61, 0x73, 0x2c, 0x20, 0x72, 0x61, 0x6e, 0x67, 0x65, 0x64, 0x20, 0x66, 0x72, 0x6f, 0x6d,
	0x20, 0x30, 0x20, 0x74, 0x6f, 0x20, 0x31, 0x2e, 0x30, 0x20, 0x28, 0x6f, 0x72, 0x20, 0x68, 0x69,
	0x67, 0x68, 0x65, 0x72, 0x29, 0x52, 0x04, 0x70, 0x65, 0x61, 0x6b, 0x2a, 0xc2, 0x01, 0x0a, 0x04,
	0x43, 0x6f, 0x64, 0x65, 0x12, 0x06, 0x0a, 0x02, 0x4f, 0x6b, 0x10, 0x00, 0x12, 0x14, 0x0a, 0x10,
	0x50, 0x65, 0x72, 0x6d, 0x69, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x44, 0x65, 0x6e, 0x69, 0x65, 0x64,
	0x10, 0x01, 0x12, 0x1c, 0x0a, 0x18, 0x49, 0x6e, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x50, 0x61, 0x72, 0x61, 0x6d, 0x65, 0x74, 0x65, 0x72, 0x73, 0x10, 0x02,
	0x12, 0x19, 0x0a, 0x15, 0x52, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x41, 0x6c, 0x72, 0x65,
	0x61, 0x64, 0x79, 0x45, 0x78, 0x69, 0x73, 0x74, 0x73, 0x10, 0x03, 0x12, 0x12, 0x0a, 0x0e, 0x54,
	0x65, 0x6e, 0x61, 0x6e, 0x74, 0x4e, 0x6f, 0x74, 0x45, 0x78, 0x69, 0x73, 0x74, 0x10, 0x04, 0x12,
	0x10, 0x0a, 0x0c, 0x54, 0x61, 0x73, 0x6b, 0x4e, 0x6f, 0x74, 0x45, 0x78, 0x69, 0x73, 0x74, 0x10,
	0x05, 0x12, 0x17, 0x0a, 0x13, 0x54, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x51, 0x75, 0x6f, 0x74, 0x61,
	0x45, 0x78, 0x63, 0x65, 0x65, 0x64, 0x65, 0x64, 0x10, 0x06, 0x12, 0x10, 0x0a, 0x0c, 0x54, 0x61,
	0x73, 0x6b, 0x46, 0x69, 0x6e, 0x69, 0x73, 0x68, 0x65, 0x64, 0x10, 0x07, 0x12, 0x12, 0x0a, 0x0e,
	0x54, 0x61, 0x73, 0x6b, 0x4e, 0x6f, 0x74, 0x52, 0x75, 0x6e, 0x6e, 0x69, 0x6e, 0x67, 0x10, 0x08,
	0x42, 0x23, 0x5a, 0x21, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6d,
	0x79, 0x6b, 0x75, 0x62, 0x65, 0x2d, 0x72, 0x75, 0x6e, 0x2f, 0x6b, 0x65, 0x65, 0x6c, 0x2f, 0x70,
	0x6b, 0x67, 0x2f, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_common_proto_rawDescOnce sync.Once
	file_common_proto_rawDescData = file_common_proto_rawDesc
)

func file_common_proto_rawDescGZIP() []byte {
	file_common_proto_rawDescOnce.Do(func() {
		file_common_proto_rawDescData = protoimpl.X.CompressGZIP(file_common_proto_rawDescData)
	})
	return file_common_proto_rawDescData
}

var file_common_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_common_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_common_proto_goTypes = []interface{}{
	(Code)(0),             // 0: pb.Code
	(*Response)(nil),      // 1: pb.Response
	(*ResourceQuota)(nil), // 2: pb.ResourceQuota
}
var file_common_proto_depIdxs = []int32{
	0, // 0: pb.Response.code:type_name -> pb.Code
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_common_proto_init() }
func file_common_proto_init() {
	if File_common_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_common_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Response); i {
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
		file_common_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ResourceQuota); i {
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
			RawDescriptor: file_common_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_common_proto_goTypes,
		DependencyIndexes: file_common_proto_depIdxs,
		EnumInfos:         file_common_proto_enumTypes,
		MessageInfos:      file_common_proto_msgTypes,
	}.Build()
	File_common_proto = out.File
	file_common_proto_rawDesc = nil
	file_common_proto_goTypes = nil
	file_common_proto_depIdxs = nil
}
