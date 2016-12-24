// Code generated by protoc-gen-go.
// source: FS.proto
// DO NOT EDIT!

package proto

import proto1 "github.com/golang/protobuf/proto"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto1.Marshal
var _ = math.Inf

type Reference_Range int32

const (
	Reference_TOP    Reference_Range = 0
	Reference_BOTTOM Reference_Range = 1
)

var Reference_Range_name = map[int32]string{
	0: "TOP",
	1: "BOTTOM",
}
var Reference_Range_value = map[string]int32{
	"TOP":    0,
	"BOTTOM": 1,
}

func (x Reference_Range) Enum() *Reference_Range {
	p := new(Reference_Range)
	*p = x
	return p
}
func (x Reference_Range) String() string {
	return proto1.EnumName(Reference_Range_name, int32(x))
}
func (x *Reference_Range) UnmarshalJSON(data []byte) error {
	value, err := proto1.UnmarshalJSONEnum(Reference_Range_value, data, "Reference_Range")
	if err != nil {
		return err
	}
	*x = Reference_Range(value)
	return nil
}

// *
// The ${HBASE_ROOTDIR}/hbase.version file content
type HBaseVersionFileContent struct {
	Version          *string `protobuf:"bytes,1,req,name=version" json:"version,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *HBaseVersionFileContent) Reset()         { *m = HBaseVersionFileContent{} }
func (m *HBaseVersionFileContent) String() string { return proto1.CompactTextString(m) }
func (*HBaseVersionFileContent) ProtoMessage()    {}

func (m *HBaseVersionFileContent) GetVersion() string {
	if m != nil && m.Version != nil {
		return *m.Version
	}
	return ""
}

// *
// Reference file content used when we split an hfile under a region.
type Reference struct {
	Splitkey         []byte           `protobuf:"bytes,1,req,name=splitkey" json:"splitkey,omitempty"`
	Range            *Reference_Range `protobuf:"varint,2,req,name=range,enum=proto.Reference_Range" json:"range,omitempty"`
	XXX_unrecognized []byte           `json:"-"`
}

func (m *Reference) Reset()         { *m = Reference{} }
func (m *Reference) String() string { return proto1.CompactTextString(m) }
func (*Reference) ProtoMessage()    {}

func (m *Reference) GetSplitkey() []byte {
	if m != nil {
		return m.Splitkey
	}
	return nil
}

func (m *Reference) GetRange() Reference_Range {
	if m != nil && m.Range != nil {
		return *m.Range
	}
	return Reference_TOP
}

func init() {
	proto1.RegisterEnum("proto.Reference_Range", Reference_Range_name, Reference_Range_value)
}
