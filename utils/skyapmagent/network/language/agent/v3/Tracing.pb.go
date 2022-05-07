// Code generated by protoc-gen-go. DO NOT EDIT.
// source: language-agent/Tracing.proto

package v3

import (
	context "context"
	fmt "fmt"
	v3 "github.com/openmsp/sidecar/utils/skyapmagent/network/common/v3"
	math "math"

	proto "github.com/golang/protobuf/proto"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = proto.Marshal
	_ = fmt.Errorf
	_ = math.Inf
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type SpanType int32

const (
	SpanType_Entry SpanType = 0
	SpanType_Exit  SpanType = 1
	SpanType_Local SpanType = 2
)

var SpanType_name = map[int32]string{
	0: "Entry",
	1: "Exit",
	2: "Local",
}

var SpanType_value = map[string]int32{
	"Entry": 0,
	"Exit":  1,
	"Local": 2,
}

func (x SpanType) String() string {
	return proto.EnumName(SpanType_name, int32(x))
}

func (SpanType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_e1989527c3daf331, []int{0}
}

type RefType int32

const (
	RefType_CrossProcess RefType = 0
	RefType_CrossThread  RefType = 1
)

var RefType_name = map[int32]string{
	0: "CrossProcess",
	1: "CrossThread",
}

var RefType_value = map[string]int32{
	"CrossProcess": 0,
	"CrossThread":  1,
}

func (x RefType) String() string {
	return proto.EnumName(RefType_name, int32(x))
}

func (RefType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_e1989527c3daf331, []int{1}
}

type SpanLayer int32

const (
	SpanLayer_Unknown      SpanLayer = 0
	SpanLayer_Database     SpanLayer = 1
	SpanLayer_RPCFramework SpanLayer = 2
	SpanLayer_Http         SpanLayer = 3
	SpanLayer_MQ           SpanLayer = 4
	SpanLayer_Cache        SpanLayer = 5
)

var SpanLayer_name = map[int32]string{
	0: "Unknown",
	1: "Database",
	2: "RPCFramework",
	3: "Http",
	4: "MQ",
	5: "Cache",
}

var SpanLayer_value = map[string]int32{
	"Unknown":      0,
	"Database":     1,
	"RPCFramework": 2,
	"Http":         3,
	"MQ":           4,
	"Cache":        5,
}

func (x SpanLayer) String() string {
	return proto.EnumName(SpanLayer_name, int32(x))
}

func (SpanLayer) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_e1989527c3daf331, []int{2}
}

type SegmentObject struct {
	TraceId              string        `protobuf:"bytes,1,opt,name=traceId,proto3" json:"traceId,omitempty"`
	TraceSegmentId       string        `protobuf:"bytes,2,opt,name=traceSegmentId,proto3" json:"traceSegmentId,omitempty"`
	Spans                []*SpanObject `protobuf:"bytes,3,rep,name=spans,proto3" json:"spans,omitempty"`
	Service              string        `protobuf:"bytes,4,opt,name=service,proto3" json:"service,omitempty"`
	ServiceInstance      string        `protobuf:"bytes,5,opt,name=serviceInstance,proto3" json:"serviceInstance,omitempty"`
	IsSizeLimited        bool          `protobuf:"varint,6,opt,name=isSizeLimited,proto3" json:"isSizeLimited,omitempty"`
	XXX_NoUnkeyedLiteral struct{}      `json:"-"`
	XXX_unrecognized     []byte        `json:"-"`
	XXX_sizecache        int32         `json:"-"`
}

func (m *SegmentObject) Reset()         { *m = SegmentObject{} }
func (m *SegmentObject) String() string { return proto.CompactTextString(m) }
func (*SegmentObject) ProtoMessage()    {}
func (*SegmentObject) Descriptor() ([]byte, []int) {
	return fileDescriptor_e1989527c3daf331, []int{0}
}

func (m *SegmentObject) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_SegmentObject.Unmarshal(m, b)
}

func (m *SegmentObject) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_SegmentObject.Marshal(b, m, deterministic)
}

func (m *SegmentObject) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SegmentObject.Merge(m, src)
}

func (m *SegmentObject) XXX_Size() int {
	return xxx_messageInfo_SegmentObject.Size(m)
}

func (m *SegmentObject) XXX_DiscardUnknown() {
	xxx_messageInfo_SegmentObject.DiscardUnknown(m)
}

var xxx_messageInfo_SegmentObject proto.InternalMessageInfo

func (m *SegmentObject) GetTraceId() string {
	if m != nil {
		return m.TraceId
	}
	return ""
}

func (m *SegmentObject) GetTraceSegmentId() string {
	if m != nil {
		return m.TraceSegmentId
	}
	return ""
}

func (m *SegmentObject) GetSpans() []*SpanObject {
	if m != nil {
		return m.Spans
	}
	return nil
}

func (m *SegmentObject) GetService() string {
	if m != nil {
		return m.Service
	}
	return ""
}

func (m *SegmentObject) GetServiceInstance() string {
	if m != nil {
		return m.ServiceInstance
	}
	return ""
}

func (m *SegmentObject) GetIsSizeLimited() bool {
	if m != nil {
		return m.IsSizeLimited
	}
	return false
}

type SegmentReference struct {
	RefType                  RefType  `protobuf:"varint,1,opt,name=refType,proto3,enum=RefType" json:"refType,omitempty"`
	TraceId                  string   `protobuf:"bytes,2,opt,name=traceId,proto3" json:"traceId,omitempty"`
	ParentTraceSegmentId     string   `protobuf:"bytes,3,opt,name=parentTraceSegmentId,proto3" json:"parentTraceSegmentId,omitempty"`
	ParentSpanId             int32    `protobuf:"varint,4,opt,name=parentSpanId,proto3" json:"parentSpanId,omitempty"`
	ParentService            string   `protobuf:"bytes,5,opt,name=parentService,proto3" json:"parentService,omitempty"`
	ParentServiceInstance    string   `protobuf:"bytes,6,opt,name=parentServiceInstance,proto3" json:"parentServiceInstance,omitempty"`
	ParentEndpoint           string   `protobuf:"bytes,7,opt,name=parentEndpoint,proto3" json:"parentEndpoint,omitempty"`
	NetworkAddressUsedAtPeer string   `protobuf:"bytes,8,opt,name=networkAddressUsedAtPeer,proto3" json:"networkAddressUsedAtPeer,omitempty"`
	XXX_NoUnkeyedLiteral     struct{} `json:"-"`
	XXX_unrecognized         []byte   `json:"-"`
	XXX_sizecache            int32    `json:"-"`
}

func (m *SegmentReference) Reset()         { *m = SegmentReference{} }
func (m *SegmentReference) String() string { return proto.CompactTextString(m) }
func (*SegmentReference) ProtoMessage()    {}
func (*SegmentReference) Descriptor() ([]byte, []int) {
	return fileDescriptor_e1989527c3daf331, []int{1}
}

func (m *SegmentReference) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_SegmentReference.Unmarshal(m, b)
}

func (m *SegmentReference) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_SegmentReference.Marshal(b, m, deterministic)
}

func (m *SegmentReference) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SegmentReference.Merge(m, src)
}

func (m *SegmentReference) XXX_Size() int {
	return xxx_messageInfo_SegmentReference.Size(m)
}

func (m *SegmentReference) XXX_DiscardUnknown() {
	xxx_messageInfo_SegmentReference.DiscardUnknown(m)
}

var xxx_messageInfo_SegmentReference proto.InternalMessageInfo

func (m *SegmentReference) GetRefType() RefType {
	if m != nil {
		return m.RefType
	}
	return RefType_CrossProcess
}

func (m *SegmentReference) GetTraceId() string {
	if m != nil {
		return m.TraceId
	}
	return ""
}

func (m *SegmentReference) GetParentTraceSegmentId() string {
	if m != nil {
		return m.ParentTraceSegmentId
	}
	return ""
}

func (m *SegmentReference) GetParentSpanId() int32 {
	if m != nil {
		return m.ParentSpanId
	}
	return 0
}

func (m *SegmentReference) GetParentService() string {
	if m != nil {
		return m.ParentService
	}
	return ""
}

func (m *SegmentReference) GetParentServiceInstance() string {
	if m != nil {
		return m.ParentServiceInstance
	}
	return ""
}

func (m *SegmentReference) GetParentEndpoint() string {
	if m != nil {
		return m.ParentEndpoint
	}
	return ""
}

func (m *SegmentReference) GetNetworkAddressUsedAtPeer() string {
	if m != nil {
		return m.NetworkAddressUsedAtPeer
	}
	return ""
}

type SpanObject struct {
	SpanId               int32                    `protobuf:"varint,1,opt,name=spanId,proto3" json:"spanId,omitempty"`
	ParentSpanId         int32                    `protobuf:"varint,2,opt,name=parentSpanId,proto3" json:"parentSpanId,omitempty"`
	StartTime            int64                    `protobuf:"varint,3,opt,name=startTime,proto3" json:"startTime,omitempty"`
	EndTime              int64                    `protobuf:"varint,4,opt,name=endTime,proto3" json:"endTime,omitempty"`
	Refs                 []*SegmentReference      `protobuf:"bytes,5,rep,name=refs,proto3" json:"refs,omitempty"`
	OperationName        string                   `protobuf:"bytes,6,opt,name=operationName,proto3" json:"operationName,omitempty"`
	Peer                 string                   `protobuf:"bytes,7,opt,name=peer,proto3" json:"peer,omitempty"`
	SpanType             SpanType                 `protobuf:"varint,8,opt,name=spanType,proto3,enum=SpanType" json:"spanType,omitempty"`
	SpanLayer            SpanLayer                `protobuf:"varint,9,opt,name=spanLayer,proto3,enum=SpanLayer" json:"spanLayer,omitempty"`
	ComponentId          int32                    `protobuf:"varint,10,opt,name=componentId,proto3" json:"componentId,omitempty"`
	IsError              bool                     `protobuf:"varint,11,opt,name=isError,proto3" json:"isError,omitempty"`
	Tags                 []*v3.KeyStringValuePair `protobuf:"bytes,12,rep,name=tags,proto3" json:"tags,omitempty"`
	Logs                 []*Log                   `protobuf:"bytes,13,rep,name=logs,proto3" json:"logs,omitempty"`
	SkipAnalysis         bool                     `protobuf:"varint,14,opt,name=skipAnalysis,proto3" json:"skipAnalysis,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                 `json:"-"`
	XXX_unrecognized     []byte                   `json:"-"`
	XXX_sizecache        int32                    `json:"-"`
}

func (m *SpanObject) Reset()         { *m = SpanObject{} }
func (m *SpanObject) String() string { return proto.CompactTextString(m) }
func (*SpanObject) ProtoMessage()    {}
func (*SpanObject) Descriptor() ([]byte, []int) {
	return fileDescriptor_e1989527c3daf331, []int{2}
}

func (m *SpanObject) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_SpanObject.Unmarshal(m, b)
}

func (m *SpanObject) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_SpanObject.Marshal(b, m, deterministic)
}

func (m *SpanObject) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SpanObject.Merge(m, src)
}

func (m *SpanObject) XXX_Size() int {
	return xxx_messageInfo_SpanObject.Size(m)
}

func (m *SpanObject) XXX_DiscardUnknown() {
	xxx_messageInfo_SpanObject.DiscardUnknown(m)
}

var xxx_messageInfo_SpanObject proto.InternalMessageInfo

func (m *SpanObject) GetSpanId() int32 {
	if m != nil {
		return m.SpanId
	}
	return 0
}

func (m *SpanObject) GetParentSpanId() int32 {
	if m != nil {
		return m.ParentSpanId
	}
	return 0
}

func (m *SpanObject) GetStartTime() int64 {
	if m != nil {
		return m.StartTime
	}
	return 0
}

func (m *SpanObject) GetEndTime() int64 {
	if m != nil {
		return m.EndTime
	}
	return 0
}

func (m *SpanObject) GetRefs() []*SegmentReference {
	if m != nil {
		return m.Refs
	}
	return nil
}

func (m *SpanObject) GetOperationName() string {
	if m != nil {
		return m.OperationName
	}
	return ""
}

func (m *SpanObject) GetPeer() string {
	if m != nil {
		return m.Peer
	}
	return ""
}

func (m *SpanObject) GetSpanType() SpanType {
	if m != nil {
		return m.SpanType
	}
	return SpanType_Entry
}

func (m *SpanObject) GetSpanLayer() SpanLayer {
	if m != nil {
		return m.SpanLayer
	}
	return SpanLayer_Unknown
}

func (m *SpanObject) GetComponentId() int32 {
	if m != nil {
		return m.ComponentId
	}
	return 0
}

func (m *SpanObject) GetIsError() bool {
	if m != nil {
		return m.IsError
	}
	return false
}

func (m *SpanObject) GetTags() []*v3.KeyStringValuePair {
	if m != nil {
		return m.Tags
	}
	return nil
}

func (m *SpanObject) GetLogs() []*Log {
	if m != nil {
		return m.Logs
	}
	return nil
}

func (m *SpanObject) GetSkipAnalysis() bool {
	if m != nil {
		return m.SkipAnalysis
	}
	return false
}

type Log struct {
	Time                 int64                    `protobuf:"varint,1,opt,name=time,proto3" json:"time,omitempty"`
	Data                 []*v3.KeyStringValuePair `protobuf:"bytes,2,rep,name=data,proto3" json:"data,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                 `json:"-"`
	XXX_unrecognized     []byte                   `json:"-"`
	XXX_sizecache        int32                    `json:"-"`
}

func (m *Log) Reset()         { *m = Log{} }
func (m *Log) String() string { return proto.CompactTextString(m) }
func (*Log) ProtoMessage()    {}
func (*Log) Descriptor() ([]byte, []int) {
	return fileDescriptor_e1989527c3daf331, []int{3}
}

func (m *Log) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Log.Unmarshal(m, b)
}

func (m *Log) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Log.Marshal(b, m, deterministic)
}

func (m *Log) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Log.Merge(m, src)
}

func (m *Log) XXX_Size() int {
	return xxx_messageInfo_Log.Size(m)
}

func (m *Log) XXX_DiscardUnknown() {
	xxx_messageInfo_Log.DiscardUnknown(m)
}

var xxx_messageInfo_Log proto.InternalMessageInfo

func (m *Log) GetTime() int64 {
	if m != nil {
		return m.Time
	}
	return 0
}

func (m *Log) GetData() []*v3.KeyStringValuePair {
	if m != nil {
		return m.Data
	}
	return nil
}

type ID struct {
	Id                   []string `protobuf:"bytes,1,rep,name=id,proto3" json:"id,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ID) Reset()         { *m = ID{} }
func (m *ID) String() string { return proto.CompactTextString(m) }
func (*ID) ProtoMessage()    {}
func (*ID) Descriptor() ([]byte, []int) {
	return fileDescriptor_e1989527c3daf331, []int{4}
}

func (m *ID) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ID.Unmarshal(m, b)
}

func (m *ID) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ID.Marshal(b, m, deterministic)
}

func (m *ID) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ID.Merge(m, src)
}

func (m *ID) XXX_Size() int {
	return xxx_messageInfo_ID.Size(m)
}

func (m *ID) XXX_DiscardUnknown() {
	xxx_messageInfo_ID.DiscardUnknown(m)
}

var xxx_messageInfo_ID proto.InternalMessageInfo

func (m *ID) GetId() []string {
	if m != nil {
		return m.Id
	}
	return nil
}

func init() {
	// proto.RegisterEnum("SpanType", SpanType_name, SpanType_value)
	// proto.RegisterEnum("RefType", RefType_name, RefType_value)
	// proto.RegisterEnum("SpanLayer", SpanLayer_name, SpanLayer_value)
	proto.RegisterType((*SegmentObject)(nil), "SegmentObject")
	proto.RegisterType((*SegmentReference)(nil), "SegmentReference")
	proto.RegisterType((*SpanObject)(nil), "SpanObject")
	proto.RegisterType((*Log)(nil), "Log")
	proto.RegisterType((*ID)(nil), "ID")
}

func init() { proto.RegisterFile("language-agent/Tracing.proto", fileDescriptor_e1989527c3daf331) }

var fileDescriptor_e1989527c3daf331 = []byte{
	// 820 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x7c, 0x54, 0x4d, 0x73, 0xe3, 0x44,
	0x10, 0x8d, 0xe4, 0xef, 0x76, 0xe2, 0x15, 0xb3, 0x0b, 0x35, 0xa4, 0xf6, 0x10, 0x5c, 0xbb, 0xe0,
	0x4a, 0x81, 0x5c, 0x95, 0x70, 0xe2, 0x96, 0xcd, 0x9a, 0xc2, 0x85, 0x59, 0x8c, 0xec, 0x40, 0x15,
	0xb7, 0x89, 0xd4, 0xd1, 0x0a, 0x4b, 0x33, 0xaa, 0x99, 0xd9, 0x04, 0xe7, 0x27, 0xf1, 0x7f, 0xb8,
	0x72, 0xe6, 0x67, 0x50, 0xd3, 0x92, 0x93, 0x28, 0x04, 0x4e, 0x9a, 0x7e, 0xef, 0x69, 0x66, 0xfa,
	0x75, 0x4f, 0xc3, 0xcb, 0x5c, 0xc8, 0xf4, 0x83, 0x48, 0xf1, 0x2b, 0x91, 0xa2, 0xb4, 0xd3, 0xb5,
	0x16, 0x71, 0x26, 0xd3, 0xb0, 0xd4, 0xca, 0xaa, 0xc3, 0xe7, 0xb1, 0x2a, 0x0a, 0x25, 0xa7, 0xe7,
	0xf4, 0xa9, 0xc0, 0xf1, 0x5f, 0x1e, 0x1c, 0xac, 0x30, 0x2d, 0x50, 0xda, 0x1f, 0x2f, 0x7f, 0xc3,
	0xd8, 0x32, 0x0e, 0x3d, 0xab, 0x45, 0x8c, 0xf3, 0x84, 0x7b, 0x47, 0xde, 0x64, 0x10, 0xed, 0x42,
	0xf6, 0x39, 0x8c, 0x68, 0x59, 0xeb, 0xe7, 0x09, 0xf7, 0x49, 0xf0, 0x08, 0x65, 0x9f, 0x41, 0xc7,
	0x94, 0x42, 0x1a, 0xde, 0x3a, 0x6a, 0x4d, 0x86, 0x27, 0xc3, 0x70, 0x55, 0x0a, 0x59, 0xed, 0x1e,
	0x55, 0x8c, 0x3b, 0xc4, 0xa0, 0xbe, 0xce, 0x62, 0xe4, 0xed, 0xea, 0x90, 0x3a, 0x64, 0x13, 0x78,
	0x56, 0x2f, 0xe7, 0xd2, 0x58, 0x21, 0x63, 0xe4, 0x1d, 0x52, 0x3c, 0x86, 0xd9, 0x2b, 0x38, 0xc8,
	0xcc, 0x2a, 0xbb, 0xc5, 0x45, 0x56, 0x64, 0x16, 0x13, 0xde, 0x3d, 0xf2, 0x26, 0xfd, 0xa8, 0x09,
	0x8e, 0xff, 0xf6, 0x21, 0xa8, 0xaf, 0x16, 0xe1, 0x15, 0x6a, 0x74, 0xbf, 0x8e, 0xa1, 0xa7, 0xf1,
	0x6a, 0xbd, 0x2d, 0x91, 0x72, 0x1c, 0x9d, 0xf4, 0xc3, 0xa8, 0x8a, 0xa3, 0x1d, 0xf1, 0xd0, 0x07,
	0xbf, 0xe9, 0xc3, 0x09, 0xbc, 0x28, 0x85, 0x46, 0x69, 0xd7, 0x4d, 0x37, 0x5a, 0x24, 0x7b, 0x92,
	0x63, 0x63, 0xd8, 0xaf, 0x70, 0xe7, 0xc5, 0x3c, 0xa1, 0xac, 0x3b, 0x51, 0x03, 0x73, 0x09, 0xd5,
	0x71, 0x6d, 0x4d, 0x95, 0x78, 0x13, 0x64, 0x5f, 0xc3, 0xc7, 0x0d, 0xe0, 0xce, 0xa6, 0x2e, 0xa9,
	0x9f, 0x26, 0x5d, 0xed, 0x2a, 0x62, 0x26, 0x93, 0x52, 0x65, 0xd2, 0xf2, 0x5e, 0x55, 0xbb, 0x26,
	0xca, 0xbe, 0x01, 0x2e, 0xd1, 0xde, 0x28, 0xbd, 0x39, 0x4b, 0x12, 0x8d, 0xc6, 0x5c, 0x18, 0x4c,
	0xce, 0xec, 0x12, 0x51, 0xf3, 0x3e, 0xfd, 0xf1, 0x9f, 0xfc, 0xf8, 0xcf, 0x16, 0xc0, 0x7d, 0xa9,
	0xd9, 0x27, 0xd0, 0x35, 0x55, 0xb2, 0x1e, 0x25, 0x5b, 0x47, 0xff, 0xb2, 0xc2, 0x7f, 0xc2, 0x8a,
	0x97, 0x30, 0x30, 0x56, 0x68, 0xbb, 0xce, 0x0a, 0x24, 0x5f, 0x5b, 0xd1, 0x3d, 0xe0, 0x4a, 0x83,
	0x32, 0x21, 0xae, 0x4d, 0xdc, 0x2e, 0x64, 0xaf, 0xa1, 0xad, 0xf1, 0xca, 0xf0, 0x0e, 0x75, 0xde,
	0x47, 0xe1, 0xe3, 0xca, 0x47, 0x44, 0x3b, 0xa7, 0x55, 0x89, 0x5a, 0xd8, 0x4c, 0xc9, 0x77, 0xa2,
	0xd8, 0x79, 0xd7, 0x04, 0x19, 0x83, 0x76, 0xe9, 0xf2, 0xae, 0x9c, 0xa2, 0x35, 0x7b, 0x0d, 0x7d,
	0x97, 0x06, 0xb5, 0x4e, 0x9f, 0x5a, 0x67, 0x40, 0xed, 0x4d, 0xbd, 0x73, 0x47, 0xb1, 0x09, 0x0c,
	0xdc, 0x7a, 0x21, 0xb6, 0xa8, 0xf9, 0x80, 0x74, 0x40, 0x3a, 0x42, 0xa2, 0x7b, 0x92, 0x1d, 0xc1,
	0x30, 0x56, 0x45, 0xa9, 0x64, 0xd5, 0x43, 0x40, 0x66, 0x3c, 0x84, 0x5c, 0xb6, 0x99, 0x99, 0x69,
	0xad, 0x34, 0x1f, 0x52, 0x87, 0xef, 0x42, 0xf6, 0x05, 0xb4, 0xad, 0x48, 0x0d, 0xdf, 0xa7, 0x6c,
	0x9f, 0x87, 0xdf, 0xe3, 0x76, 0x65, 0x75, 0x26, 0xd3, 0x9f, 0x45, 0xfe, 0x01, 0x97, 0x22, 0xd3,
	0x11, 0x09, 0x18, 0x87, 0x76, 0xae, 0x52, 0xc3, 0x0f, 0x48, 0xd8, 0x0e, 0x17, 0x2a, 0x8d, 0x08,
	0x71, 0xc5, 0x30, 0x9b, 0xac, 0x3c, 0x93, 0x22, 0xdf, 0x9a, 0xcc, 0xf0, 0x11, 0x9d, 0xd0, 0xc0,
	0xc6, 0x6f, 0xa0, 0xb5, 0x50, 0xa9, 0xb3, 0xc3, 0x3a, 0xcb, 0x3d, 0xb2, 0x9c, 0xd6, 0xee, 0x06,
	0x89, 0xb0, 0x82, 0xfb, 0xff, 0x73, 0x03, 0x27, 0x18, 0xbf, 0x00, 0x7f, 0xfe, 0x96, 0x8d, 0xc0,
	0xcf, 0x5c, 0x3b, 0xb4, 0x26, 0x83, 0xc8, 0xcf, 0x92, 0xe3, 0x63, 0xe8, 0xef, 0xcc, 0x63, 0x03,
	0xe8, 0xcc, 0xa4, 0xd5, 0xdb, 0x60, 0x8f, 0xf5, 0xa1, 0x3d, 0xfb, 0x3d, 0xb3, 0x81, 0xe7, 0xc0,
	0x85, 0x8a, 0x45, 0x1e, 0xf8, 0xc7, 0x5f, 0x42, 0xaf, 0x7e, 0xa3, 0x2c, 0x80, 0xfd, 0x73, 0xad,
	0x8c, 0x59, 0x6a, 0x15, 0xa3, 0x31, 0xc1, 0x1e, 0x7b, 0x06, 0x43, 0x42, 0xd6, 0xef, 0x35, 0x8a,
	0x24, 0xf0, 0x8e, 0x2f, 0x60, 0x70, 0x67, 0x37, 0x1b, 0x42, 0xef, 0x42, 0x6e, 0xa4, 0xba, 0x91,
	0xc1, 0x1e, 0xdb, 0x87, 0xfe, 0x5b, 0x61, 0xc5, 0xa5, 0x30, 0x18, 0x78, 0x6e, 0xab, 0x68, 0x79,
	0xfe, 0xad, 0x16, 0x05, 0xba, 0xa6, 0x0e, 0x7c, 0x77, 0xf8, 0x77, 0xd6, 0x96, 0x41, 0x8b, 0x75,
	0xc1, 0xff, 0xe1, 0xa7, 0xa0, 0xed, 0x2e, 0x71, 0x2e, 0xe2, 0xf7, 0x18, 0x74, 0x4e, 0x66, 0xf0,
	0xe9, 0xc3, 0x87, 0x1d, 0x61, 0xa9, 0xf4, 0xdd, 0xcb, 0x9c, 0x40, 0x2f, 0x56, 0x79, 0xee, 0x7a,
	0x7f, 0x14, 0x36, 0x86, 0xea, 0xe1, 0x20, 0x74, 0x53, 0x57, 0xc8, 0xc4, 0x8c, 0xf7, 0x26, 0xde,
	0x9b, 0x5b, 0x38, 0x55, 0x3a, 0x0d, 0x45, 0xe9, 0xb6, 0x0d, 0xcd, 0x66, 0x7b, 0x23, 0xf2, 0x8d,
	0x9b, 0xd4, 0xa2, 0x2c, 0xc2, 0xfa, 0x7d, 0x85, 0xbb, 0x61, 0x1e, 0xd2, 0x30, 0x0f, 0xaf, 0x4f,
	0x97, 0xde, 0xaf, 0xaf, 0xee, 0xb5, 0xd3, 0x5a, 0x37, 0xdd, 0xe9, 0xa6, 0xd5, 0xd0, 0xbf, 0x3e,
	0xfd, 0xc3, 0x3f, 0x5c, 0x6d, 0xb6, 0xbf, 0xd4, 0x5b, 0xbe, 0xab, 0x64, 0x4b, 0x37, 0xee, 0x63,
	0x95, 0x5f, 0x76, 0x69, 0xf0, 0x9f, 0xfe, 0x13, 0x00, 0x00, 0xff, 0xff, 0xb3, 0xf3, 0x2d, 0x5b,
	0x2d, 0x06, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ context.Context
	_ grpc.ClientConn
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// TraceSegmentReportServiceClient is the client API for TraceSegmentReportService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type TraceSegmentReportServiceClient interface {
	Collect(ctx context.Context, opts ...grpc.CallOption) (TraceSegmentReportService_CollectClient, error)
}

type traceSegmentReportServiceClient struct {
	cc *grpc.ClientConn
}

func NewTraceSegmentReportServiceClient(cc *grpc.ClientConn) TraceSegmentReportServiceClient {
	return &traceSegmentReportServiceClient{cc}
}

func (c *traceSegmentReportServiceClient) Collect(ctx context.Context, opts ...grpc.CallOption) (TraceSegmentReportService_CollectClient, error) {
	stream, err := c.cc.NewStream(ctx, &_TraceSegmentReportService_serviceDesc.Streams[0], "/TraceSegmentReportService/collect", opts...)
	if err != nil {
		return nil, err
	}
	x := &traceSegmentReportServiceCollectClient{stream}
	return x, nil
}

type TraceSegmentReportService_CollectClient interface {
	Send(*SegmentObject) error
	CloseAndRecv() (*v3.Commands, error)
	grpc.ClientStream
}

type traceSegmentReportServiceCollectClient struct {
	grpc.ClientStream
}

func (x *traceSegmentReportServiceCollectClient) Send(m *SegmentObject) error {
	return x.ClientStream.SendMsg(m)
}

func (x *traceSegmentReportServiceCollectClient) CloseAndRecv() (*v3.Commands, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(v3.Commands)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// TraceSegmentReportServiceServer is the server API for TraceSegmentReportService service.
type TraceSegmentReportServiceServer interface {
	Collect(TraceSegmentReportService_CollectServer) error
}

// UnimplementedTraceSegmentReportServiceServer can be embedded to have forward compatible implementations.
type UnimplementedTraceSegmentReportServiceServer struct{}

func (*UnimplementedTraceSegmentReportServiceServer) Collect(srv TraceSegmentReportService_CollectServer) error {
	return status.Errorf(codes.Unimplemented, "method Collect not implemented")
}

func RegisterTraceSegmentReportServiceServer(s *grpc.Server, srv TraceSegmentReportServiceServer) {
	s.RegisterService(&_TraceSegmentReportService_serviceDesc, srv)
}

func _TraceSegmentReportService_Collect_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(TraceSegmentReportServiceServer).Collect(&traceSegmentReportServiceCollectServer{stream})
}

type TraceSegmentReportService_CollectServer interface {
	SendAndClose(*v3.Commands) error
	Recv() (*SegmentObject, error)
	grpc.ServerStream
}

type traceSegmentReportServiceCollectServer struct {
	grpc.ServerStream
}

func (x *traceSegmentReportServiceCollectServer) SendAndClose(m *v3.Commands) error {
	return x.ServerStream.SendMsg(m)
}

func (x *traceSegmentReportServiceCollectServer) Recv() (*SegmentObject, error) {
	m := new(SegmentObject)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var _TraceSegmentReportService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "TraceSegmentReportService",
	HandlerType: (*TraceSegmentReportServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "collect",
			Handler:       _TraceSegmentReportService_Collect_Handler,
			ClientStreams: true,
		},
	},
	Metadata: "language-agent/Tracing.proto",
}
