// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        v3.20.0
// source: store/storepb/types.proto

package storepb

import (
	reflect "reflect"
	sync "sync"

	_ "github.com/planetscale/vtprotobuf/vtproto"
	labelpb "github.com/thanos-io/thanos/pkg/store/labelpb"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// / PartialResponseStrategy controls partial response handling.
type PartialResponseStrategy int32

const (
	// / WARN strategy tells server to treat any error that will related to single StoreAPI (e.g missing chunk series because of underlying
	// / storeAPI is temporarily not available) as warning which will not fail the whole query (still OK response).
	// / Server should produce those as a warnings field in response.
	PartialResponseStrategy_WARN PartialResponseStrategy = 0
	// / ABORT strategy tells server to treat any error that will related to single StoreAPI (e.g missing chunk series because of underlying
	// / storeAPI is temporarily not available) as the gRPC error that aborts the query.
	// /
	// / This is especially useful for any rule/alert evaluations on top of StoreAPI which usually does not tolerate partial
	// / errors.
	PartialResponseStrategy_ABORT PartialResponseStrategy = 1
	/// A group has one or more replicas. A replica has one or more endpoints.
	/// If a group has one replica, the replica can only tolerate one endpoint failure.
	/// If a group has more than one replicas, the group can tolerate any number of endpoint failures wihtin one replica. It doens't
	///   tolerate endpoint failures across replicas.
	PartialResponseStrategy_GROUP_REPLICA PartialResponseStrategy = 2
)

// Enum value maps for PartialResponseStrategy.
var (
	PartialResponseStrategy_name = map[int32]string{
		0: "WARN",
		1: "ABORT",
	}
	PartialResponseStrategy_value = map[string]int32{
		"WARN":  0,
		"ABORT": 1,
	}
)

func (x PartialResponseStrategy) Enum() *PartialResponseStrategy {
	p := new(PartialResponseStrategy)
	*p = x
	return p
}

func (x PartialResponseStrategy) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (PartialResponseStrategy) Descriptor() protoreflect.EnumDescriptor {
	return file_store_storepb_types_proto_enumTypes[0].Descriptor()
}

func (PartialResponseStrategy) Type() protoreflect.EnumType {
	return &file_store_storepb_types_proto_enumTypes[0]
}

func (x PartialResponseStrategy) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use PartialResponseStrategy.Descriptor instead.
func (PartialResponseStrategy) EnumDescriptor() ([]byte, []int) {
	return file_store_storepb_types_proto_rawDescGZIP(), []int{0}
}

type Chunk_Encoding int32

const (
	Chunk_XOR             Chunk_Encoding = 0
	Chunk_HISTOGRAM       Chunk_Encoding = 1
	Chunk_FLOAT_HISTOGRAM Chunk_Encoding = 2
)

// Enum value maps for Chunk_Encoding.
var (
	Chunk_Encoding_name = map[int32]string{
		0: "XOR",
		1: "HISTOGRAM",
		2: "FLOAT_HISTOGRAM",
	}
	Chunk_Encoding_value = map[string]int32{
		"XOR":             0,
		"HISTOGRAM":       1,
		"FLOAT_HISTOGRAM": 2,
	}
)

func (x Chunk_Encoding) Enum() *Chunk_Encoding {
	p := new(Chunk_Encoding)
	*p = x
	return p
}

func (x Chunk_Encoding) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Chunk_Encoding) Descriptor() protoreflect.EnumDescriptor {
	return file_store_storepb_types_proto_enumTypes[1].Descriptor()
}

func (Chunk_Encoding) Type() protoreflect.EnumType {
	return &file_store_storepb_types_proto_enumTypes[1]
}

func (x Chunk_Encoding) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Chunk_Encoding.Descriptor instead.
func (Chunk_Encoding) EnumDescriptor() ([]byte, []int) {
	return file_store_storepb_types_proto_rawDescGZIP(), []int{0, 0}
}

type LabelMatcher_Type int32

const (
	LabelMatcher_EQ  LabelMatcher_Type = 0 // =
	LabelMatcher_NEQ LabelMatcher_Type = 1 // !=
	LabelMatcher_RE  LabelMatcher_Type = 2 // =~
	LabelMatcher_NRE LabelMatcher_Type = 3 // !~
)

// Enum value maps for LabelMatcher_Type.
var (
	LabelMatcher_Type_name = map[int32]string{
		0: "EQ",
		1: "NEQ",
		2: "RE",
		3: "NRE",
	}
	LabelMatcher_Type_value = map[string]int32{
		"EQ":  0,
		"NEQ": 1,
		"RE":  2,
		"NRE": 3,
	}
)

func (x LabelMatcher_Type) Enum() *LabelMatcher_Type {
	p := new(LabelMatcher_Type)
	*p = x
	return p
}

func (x LabelMatcher_Type) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (LabelMatcher_Type) Descriptor() protoreflect.EnumDescriptor {
	return file_store_storepb_types_proto_enumTypes[2].Descriptor()
}

func (LabelMatcher_Type) Type() protoreflect.EnumType {
	return &file_store_storepb_types_proto_enumTypes[2]
}

func (x LabelMatcher_Type) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use LabelMatcher_Type.Descriptor instead.
func (LabelMatcher_Type) EnumDescriptor() ([]byte, []int) {
	return file_store_storepb_types_proto_rawDescGZIP(), []int{3, 0}
}

type Chunk struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Type Chunk_Encoding `protobuf:"varint,1,opt,name=type,proto3,enum=thanos.Chunk_Encoding" json:"type,omitempty"`
	Data []byte         `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
	Hash uint64         `protobuf:"varint,3,opt,name=hash,proto3" json:"hash,omitempty"`
}

func (x *Chunk) Reset() {
	*x = Chunk{}
	if protoimpl.UnsafeEnabled {
		mi := &file_store_storepb_types_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Chunk) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Chunk) ProtoMessage() {}

func (x *Chunk) ProtoReflect() protoreflect.Message {
	mi := &file_store_storepb_types_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Chunk.ProtoReflect.Descriptor instead.
func (*Chunk) Descriptor() ([]byte, []int) {
	return file_store_storepb_types_proto_rawDescGZIP(), []int{0}
}

func (x *Chunk) GetType() Chunk_Encoding {
	if x != nil {
		return x.Type
	}
	return Chunk_XOR
}

func (x *Chunk) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *Chunk) GetHash() uint64 {
	if x != nil {
		return x.Hash
	}
	return 0
}

type Series struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Labels []*labelpb.Label `protobuf:"bytes,1,rep,name=labels,proto3" json:"labels,omitempty"`
	Chunks []*AggrChunk     `protobuf:"bytes,2,rep,name=chunks,proto3" json:"chunks,omitempty"`
}

func (x *Series) Reset() {
	*x = Series{}
	if protoimpl.UnsafeEnabled {
		mi := &file_store_storepb_types_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Series) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Series) ProtoMessage() {}

func (x *Series) ProtoReflect() protoreflect.Message {
	mi := &file_store_storepb_types_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Series.ProtoReflect.Descriptor instead.
func (*Series) Descriptor() ([]byte, []int) {
	return file_store_storepb_types_proto_rawDescGZIP(), []int{1}
}

func (x *Series) GetLabels() []*labelpb.Label {
	if x != nil {
		return x.Labels
	}
	return nil
}

func (x *Series) GetChunks() []*AggrChunk {
	if x != nil {
		return x.Chunks
	}
	return nil
}

type AggrChunk struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	MinTime int64  `protobuf:"varint,1,opt,name=min_time,json=minTime,proto3" json:"min_time,omitempty"`
	MaxTime int64  `protobuf:"varint,2,opt,name=max_time,json=maxTime,proto3" json:"max_time,omitempty"`
	Raw     *Chunk `protobuf:"bytes,3,opt,name=raw,proto3" json:"raw,omitempty"`
	Count   *Chunk `protobuf:"bytes,4,opt,name=count,proto3" json:"count,omitempty"`
	Sum     *Chunk `protobuf:"bytes,5,opt,name=sum,proto3" json:"sum,omitempty"`
	Min     *Chunk `protobuf:"bytes,6,opt,name=min,proto3" json:"min,omitempty"`
	Max     *Chunk `protobuf:"bytes,7,opt,name=max,proto3" json:"max,omitempty"`
	Counter *Chunk `protobuf:"bytes,8,opt,name=counter,proto3" json:"counter,omitempty"`
}

func (x *AggrChunk) Reset() {
	*x = AggrChunk{}
	if protoimpl.UnsafeEnabled {
		mi := &file_store_storepb_types_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AggrChunk) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AggrChunk) ProtoMessage() {}

func (x *AggrChunk) ProtoReflect() protoreflect.Message {
	mi := &file_store_storepb_types_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AggrChunk.ProtoReflect.Descriptor instead.
func (*AggrChunk) Descriptor() ([]byte, []int) {
	return file_store_storepb_types_proto_rawDescGZIP(), []int{2}
}

func (x *AggrChunk) GetMinTime() int64 {
	if x != nil {
		return x.MinTime
	}
	return 0
}

func (x *AggrChunk) GetMaxTime() int64 {
	if x != nil {
		return x.MaxTime
	}
	return 0
}

func (x *AggrChunk) GetRaw() *Chunk {
	if x != nil {
		return x.Raw
	}
	return nil
}

func (x *AggrChunk) GetCount() *Chunk {
	if x != nil {
		return x.Count
	}
	return nil
}

func (x *AggrChunk) GetSum() *Chunk {
	if x != nil {
		return x.Sum
	}
	return nil
}

func (x *AggrChunk) GetMin() *Chunk {
	if x != nil {
		return x.Min
	}
	return nil
}

func (x *AggrChunk) GetMax() *Chunk {
	if x != nil {
		return x.Max
	}
	return nil
}

func (x *AggrChunk) GetCounter() *Chunk {
	if x != nil {
		return x.Counter
	}
	return nil
}

// Matcher specifies a rule, which can match or set of labels or not.
type LabelMatcher struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Type  LabelMatcher_Type `protobuf:"varint,1,opt,name=type,proto3,enum=thanos.LabelMatcher_Type" json:"type,omitempty"`
	Name  string            `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Value string            `protobuf:"bytes,3,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *LabelMatcher) Reset() {
	*x = LabelMatcher{}
	if protoimpl.UnsafeEnabled {
		mi := &file_store_storepb_types_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *LabelMatcher) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LabelMatcher) ProtoMessage() {}

func (x *LabelMatcher) ProtoReflect() protoreflect.Message {
	mi := &file_store_storepb_types_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LabelMatcher.ProtoReflect.Descriptor instead.
func (*LabelMatcher) Descriptor() ([]byte, []int) {
	return file_store_storepb_types_proto_rawDescGZIP(), []int{3}
}

func (x *LabelMatcher) GetType() LabelMatcher_Type {
	if x != nil {
		return x.Type
	}
	return LabelMatcher_EQ
}

func (x *LabelMatcher) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *LabelMatcher) GetValue() string {
	if x != nil {
		return x.Value
	}
	return ""
}

var File_store_storepb_types_proto protoreflect.FileDescriptor

var file_store_storepb_types_proto_rawDesc = []byte{
	0x0a, 0x19, 0x73, 0x74, 0x6f, 0x72, 0x65, 0x2f, 0x73, 0x74, 0x6f, 0x72, 0x65, 0x70, 0x62, 0x2f,
	0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x06, 0x74, 0x68, 0x61,
	0x6e, 0x6f, 0x73, 0x1a, 0x19, 0x73, 0x74, 0x6f, 0x72, 0x65, 0x2f, 0x6c, 0x61, 0x62, 0x65, 0x6c,
	0x70, 0x62, 0x2f, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x33,
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x70, 0x6c, 0x61, 0x6e, 0x65,
	0x74, 0x73, 0x63, 0x61, 0x6c, 0x65, 0x2f, 0x76, 0x74, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75,
	0x66, 0x2f, 0x76, 0x74, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x65, 0x78, 0x74, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x22, 0x9a, 0x01, 0x0a, 0x05, 0x43, 0x68, 0x75, 0x6e, 0x6b, 0x12, 0x2a, 0x0a,
	0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x16, 0x2e, 0x74, 0x68,
	0x61, 0x6e, 0x6f, 0x73, 0x2e, 0x43, 0x68, 0x75, 0x6e, 0x6b, 0x2e, 0x45, 0x6e, 0x63, 0x6f, 0x64,
	0x69, 0x6e, 0x67, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74,
	0x61, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x12, 0x12, 0x0a,
	0x04, 0x68, 0x61, 0x73, 0x68, 0x18, 0x03, 0x20, 0x01, 0x28, 0x04, 0x52, 0x04, 0x68, 0x61, 0x73,
	0x68, 0x22, 0x37, 0x0a, 0x08, 0x45, 0x6e, 0x63, 0x6f, 0x64, 0x69, 0x6e, 0x67, 0x12, 0x07, 0x0a,
	0x03, 0x58, 0x4f, 0x52, 0x10, 0x00, 0x12, 0x0d, 0x0a, 0x09, 0x48, 0x49, 0x53, 0x54, 0x4f, 0x47,
	0x52, 0x41, 0x4d, 0x10, 0x01, 0x12, 0x13, 0x0a, 0x0f, 0x46, 0x4c, 0x4f, 0x41, 0x54, 0x5f, 0x48,
	0x49, 0x53, 0x54, 0x4f, 0x47, 0x52, 0x41, 0x4d, 0x10, 0x02, 0x3a, 0x04, 0xa8, 0xa6, 0x1f, 0x01,
	0x22, 0x5a, 0x0a, 0x06, 0x53, 0x65, 0x72, 0x69, 0x65, 0x73, 0x12, 0x25, 0x0a, 0x06, 0x6c, 0x61,
	0x62, 0x65, 0x6c, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x0d, 0x2e, 0x74, 0x68, 0x61,
	0x6e, 0x6f, 0x73, 0x2e, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x52, 0x06, 0x6c, 0x61, 0x62, 0x65, 0x6c,
	0x73, 0x12, 0x29, 0x0a, 0x06, 0x63, 0x68, 0x75, 0x6e, 0x6b, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x11, 0x2e, 0x74, 0x68, 0x61, 0x6e, 0x6f, 0x73, 0x2e, 0x41, 0x67, 0x67, 0x72, 0x43,
	0x68, 0x75, 0x6e, 0x6b, 0x52, 0x06, 0x63, 0x68, 0x75, 0x6e, 0x6b, 0x73, 0x22, 0x99, 0x02, 0x0a,
	0x09, 0x41, 0x67, 0x67, 0x72, 0x43, 0x68, 0x75, 0x6e, 0x6b, 0x12, 0x19, 0x0a, 0x08, 0x6d, 0x69,
	0x6e, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x6d, 0x69,
	0x6e, 0x54, 0x69, 0x6d, 0x65, 0x12, 0x19, 0x0a, 0x08, 0x6d, 0x61, 0x78, 0x5f, 0x74, 0x69, 0x6d,
	0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x6d, 0x61, 0x78, 0x54, 0x69, 0x6d, 0x65,
	0x12, 0x1f, 0x0a, 0x03, 0x72, 0x61, 0x77, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0d, 0x2e,
	0x74, 0x68, 0x61, 0x6e, 0x6f, 0x73, 0x2e, 0x43, 0x68, 0x75, 0x6e, 0x6b, 0x52, 0x03, 0x72, 0x61,
	0x77, 0x12, 0x23, 0x0a, 0x05, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x0d, 0x2e, 0x74, 0x68, 0x61, 0x6e, 0x6f, 0x73, 0x2e, 0x43, 0x68, 0x75, 0x6e, 0x6b, 0x52,
	0x05, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x1f, 0x0a, 0x03, 0x73, 0x75, 0x6d, 0x18, 0x05, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x0d, 0x2e, 0x74, 0x68, 0x61, 0x6e, 0x6f, 0x73, 0x2e, 0x43, 0x68, 0x75,
	0x6e, 0x6b, 0x52, 0x03, 0x73, 0x75, 0x6d, 0x12, 0x1f, 0x0a, 0x03, 0x6d, 0x69, 0x6e, 0x18, 0x06,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x0d, 0x2e, 0x74, 0x68, 0x61, 0x6e, 0x6f, 0x73, 0x2e, 0x43, 0x68,
	0x75, 0x6e, 0x6b, 0x52, 0x03, 0x6d, 0x69, 0x6e, 0x12, 0x1f, 0x0a, 0x03, 0x6d, 0x61, 0x78, 0x18,
	0x07, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0d, 0x2e, 0x74, 0x68, 0x61, 0x6e, 0x6f, 0x73, 0x2e, 0x43,
	0x68, 0x75, 0x6e, 0x6b, 0x52, 0x03, 0x6d, 0x61, 0x78, 0x12, 0x27, 0x0a, 0x07, 0x63, 0x6f, 0x75,
	0x6e, 0x74, 0x65, 0x72, 0x18, 0x08, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0d, 0x2e, 0x74, 0x68, 0x61,
	0x6e, 0x6f, 0x73, 0x2e, 0x43, 0x68, 0x75, 0x6e, 0x6b, 0x52, 0x07, 0x63, 0x6f, 0x75, 0x6e, 0x74,
	0x65, 0x72, 0x3a, 0x04, 0xa8, 0xa6, 0x1f, 0x01, 0x22, 0x91, 0x01, 0x0a, 0x0c, 0x4c, 0x61, 0x62,
	0x65, 0x6c, 0x4d, 0x61, 0x74, 0x63, 0x68, 0x65, 0x72, 0x12, 0x2d, 0x0a, 0x04, 0x74, 0x79, 0x70,
	0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x19, 0x2e, 0x74, 0x68, 0x61, 0x6e, 0x6f, 0x73,
	0x2e, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x4d, 0x61, 0x74, 0x63, 0x68, 0x65, 0x72, 0x2e, 0x54, 0x79,
	0x70, 0x65, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x14, 0x0a, 0x05,
	0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c,
	0x75, 0x65, 0x22, 0x28, 0x0a, 0x04, 0x54, 0x79, 0x70, 0x65, 0x12, 0x06, 0x0a, 0x02, 0x45, 0x51,
	0x10, 0x00, 0x12, 0x07, 0x0a, 0x03, 0x4e, 0x45, 0x51, 0x10, 0x01, 0x12, 0x06, 0x0a, 0x02, 0x52,
	0x45, 0x10, 0x02, 0x12, 0x07, 0x0a, 0x03, 0x4e, 0x52, 0x45, 0x10, 0x03, 0x2a, 0x2e, 0x0a, 0x17,
	0x50, 0x61, 0x72, 0x74, 0x69, 0x61, 0x6c, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x53,
	0x74, 0x72, 0x61, 0x74, 0x65, 0x67, 0x79, 0x12, 0x08, 0x0a, 0x04, 0x57, 0x41, 0x52, 0x4e, 0x10,
	0x00, 0x12, 0x09, 0x0a, 0x05, 0x41, 0x42, 0x4f, 0x52, 0x54, 0x10, 0x01, 0x42, 0x2f, 0x5a, 0x2d,
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x74, 0x68, 0x61, 0x6e, 0x6f,
	0x73, 0x2d, 0x69, 0x6f, 0x2f, 0x74, 0x68, 0x61, 0x6e, 0x6f, 0x73, 0x2f, 0x70, 0x6b, 0x67, 0x2f,
	0x73, 0x74, 0x6f, 0x72, 0x65, 0x2f, 0x73, 0x74, 0x6f, 0x72, 0x65, 0x70, 0x62, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_store_storepb_types_proto_rawDescOnce sync.Once
	file_store_storepb_types_proto_rawDescData = file_store_storepb_types_proto_rawDesc
)

func file_store_storepb_types_proto_rawDescGZIP() []byte {
	file_store_storepb_types_proto_rawDescOnce.Do(func() {
		file_store_storepb_types_proto_rawDescData = protoimpl.X.CompressGZIP(file_store_storepb_types_proto_rawDescData)
	})
	return file_store_storepb_types_proto_rawDescData
}

var file_store_storepb_types_proto_enumTypes = make([]protoimpl.EnumInfo, 3)
var file_store_storepb_types_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_store_storepb_types_proto_goTypes = []any{
	(PartialResponseStrategy)(0), // 0: thanos.PartialResponseStrategy
	(Chunk_Encoding)(0),          // 1: thanos.Chunk.Encoding
	(LabelMatcher_Type)(0),       // 2: thanos.LabelMatcher.Type
	(*Chunk)(nil),                // 3: thanos.Chunk
	(*Series)(nil),               // 4: thanos.Series
	(*AggrChunk)(nil),            // 5: thanos.AggrChunk
	(*LabelMatcher)(nil),         // 6: thanos.LabelMatcher
	(*labelpb.Label)(nil),        // 7: thanos.Label
}
var file_store_storepb_types_proto_depIdxs = []int32{
	1,  // 0: thanos.Chunk.type:type_name -> thanos.Chunk.Encoding
	7,  // 1: thanos.Series.labels:type_name -> thanos.Label
	5,  // 2: thanos.Series.chunks:type_name -> thanos.AggrChunk
	3,  // 3: thanos.AggrChunk.raw:type_name -> thanos.Chunk
	3,  // 4: thanos.AggrChunk.count:type_name -> thanos.Chunk
	3,  // 5: thanos.AggrChunk.sum:type_name -> thanos.Chunk
	3,  // 6: thanos.AggrChunk.min:type_name -> thanos.Chunk
	3,  // 7: thanos.AggrChunk.max:type_name -> thanos.Chunk
	3,  // 8: thanos.AggrChunk.counter:type_name -> thanos.Chunk
	2,  // 9: thanos.LabelMatcher.type:type_name -> thanos.LabelMatcher.Type
	10, // [10:10] is the sub-list for method output_type
	10, // [10:10] is the sub-list for method input_type
	10, // [10:10] is the sub-list for extension type_name
	10, // [10:10] is the sub-list for extension extendee
	0,  // [0:10] is the sub-list for field type_name
}

func init() { file_store_storepb_types_proto_init() }
func file_store_storepb_types_proto_init() {
	if File_store_storepb_types_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_store_storepb_types_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*Chunk); i {
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
		file_store_storepb_types_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*Series); i {
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
		file_store_storepb_types_proto_msgTypes[2].Exporter = func(v any, i int) any {
			switch v := v.(*AggrChunk); i {
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
		file_store_storepb_types_proto_msgTypes[3].Exporter = func(v any, i int) any {
			switch v := v.(*LabelMatcher); i {
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
			RawDescriptor: file_store_storepb_types_proto_rawDesc,
			NumEnums:      3,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_store_storepb_types_proto_goTypes,
		DependencyIndexes: file_store_storepb_types_proto_depIdxs,
		EnumInfos:         file_store_storepb_types_proto_enumTypes,
		MessageInfos:      file_store_storepb_types_proto_msgTypes,
	}.Build()
	File_store_storepb_types_proto = out.File
	file_store_storepb_types_proto_rawDesc = nil
	file_store_storepb_types_proto_goTypes = nil
	file_store_storepb_types_proto_depIdxs = nil
}
