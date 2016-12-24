// Code generated by protoc-gen-go.
// source: Comparator.proto
// DO NOT EDIT!

package proto

import proto1 "github.com/golang/protobuf/proto"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto1.Marshal
var _ = math.Inf

type BitComparator_BitwiseOp int32

const (
	BitComparator_AND BitComparator_BitwiseOp = 1
	BitComparator_OR  BitComparator_BitwiseOp = 2
	BitComparator_XOR BitComparator_BitwiseOp = 3
)

var BitComparator_BitwiseOp_name = map[int32]string{
	1: "AND",
	2: "OR",
	3: "XOR",
}
var BitComparator_BitwiseOp_value = map[string]int32{
	"AND": 1,
	"OR":  2,
	"XOR": 3,
}

func (x BitComparator_BitwiseOp) Enum() *BitComparator_BitwiseOp {
	p := new(BitComparator_BitwiseOp)
	*p = x
	return p
}
func (x BitComparator_BitwiseOp) String() string {
	return proto1.EnumName(BitComparator_BitwiseOp_name, int32(x))
}
func (x *BitComparator_BitwiseOp) UnmarshalJSON(data []byte) error {
	value, err := proto1.UnmarshalJSONEnum(BitComparator_BitwiseOp_value, data, "BitComparator_BitwiseOp")
	if err != nil {
		return err
	}
	*x = BitComparator_BitwiseOp(value)
	return nil
}

type Comparator struct {
	Name                 *string `protobuf:"bytes,1,req,name=name" json:"name,omitempty"`
	SerializedComparator []byte  `protobuf:"bytes,2,opt,name=serialized_comparator" json:"serialized_comparator,omitempty"`
	XXX_unrecognized     []byte  `json:"-"`
}

func (m *Comparator) Reset()         { *m = Comparator{} }
func (m *Comparator) String() string { return proto1.CompactTextString(m) }
func (*Comparator) ProtoMessage()    {}

func (m *Comparator) GetName() string {
	if m != nil && m.Name != nil {
		return *m.Name
	}
	return ""
}

func (m *Comparator) GetSerializedComparator() []byte {
	if m != nil {
		return m.SerializedComparator
	}
	return nil
}

type ByteArrayComparable struct {
	Value            []byte `protobuf:"bytes,1,opt,name=value" json:"value,omitempty"`
	XXX_unrecognized []byte `json:"-"`
}

func (m *ByteArrayComparable) Reset()         { *m = ByteArrayComparable{} }
func (m *ByteArrayComparable) String() string { return proto1.CompactTextString(m) }
func (*ByteArrayComparable) ProtoMessage()    {}

func (m *ByteArrayComparable) GetValue() []byte {
	if m != nil {
		return m.Value
	}
	return nil
}

type BinaryComparator struct {
	Comparable       *ByteArrayComparable `protobuf:"bytes,1,req,name=comparable" json:"comparable,omitempty"`
	XXX_unrecognized []byte               `json:"-"`
}

func (m *BinaryComparator) Reset()         { *m = BinaryComparator{} }
func (m *BinaryComparator) String() string { return proto1.CompactTextString(m) }
func (*BinaryComparator) ProtoMessage()    {}

func (m *BinaryComparator) GetComparable() *ByteArrayComparable {
	if m != nil {
		return m.Comparable
	}
	return nil
}

type LongComparator struct {
	Comparable       *ByteArrayComparable `protobuf:"bytes,1,req,name=comparable" json:"comparable,omitempty"`
	XXX_unrecognized []byte               `json:"-"`
}

func (m *LongComparator) Reset()         { *m = LongComparator{} }
func (m *LongComparator) String() string { return proto1.CompactTextString(m) }
func (*LongComparator) ProtoMessage()    {}

func (m *LongComparator) GetComparable() *ByteArrayComparable {
	if m != nil {
		return m.Comparable
	}
	return nil
}

type BinaryPrefixComparator struct {
	Comparable       *ByteArrayComparable `protobuf:"bytes,1,req,name=comparable" json:"comparable,omitempty"`
	XXX_unrecognized []byte               `json:"-"`
}

func (m *BinaryPrefixComparator) Reset()         { *m = BinaryPrefixComparator{} }
func (m *BinaryPrefixComparator) String() string { return proto1.CompactTextString(m) }
func (*BinaryPrefixComparator) ProtoMessage()    {}

func (m *BinaryPrefixComparator) GetComparable() *ByteArrayComparable {
	if m != nil {
		return m.Comparable
	}
	return nil
}

type BitComparator struct {
	Comparable       *ByteArrayComparable     `protobuf:"bytes,1,req,name=comparable" json:"comparable,omitempty"`
	BitwiseOp        *BitComparator_BitwiseOp `protobuf:"varint,2,req,name=bitwise_op,enum=proto.BitComparator_BitwiseOp" json:"bitwise_op,omitempty"`
	XXX_unrecognized []byte                   `json:"-"`
}

func (m *BitComparator) Reset()         { *m = BitComparator{} }
func (m *BitComparator) String() string { return proto1.CompactTextString(m) }
func (*BitComparator) ProtoMessage()    {}

func (m *BitComparator) GetComparable() *ByteArrayComparable {
	if m != nil {
		return m.Comparable
	}
	return nil
}

func (m *BitComparator) GetBitwiseOp() BitComparator_BitwiseOp {
	if m != nil && m.BitwiseOp != nil {
		return *m.BitwiseOp
	}
	return BitComparator_AND
}

type NullComparator struct {
	XXX_unrecognized []byte `json:"-"`
}

func (m *NullComparator) Reset()         { *m = NullComparator{} }
func (m *NullComparator) String() string { return proto1.CompactTextString(m) }
func (*NullComparator) ProtoMessage()    {}

type RegexStringComparator struct {
	Pattern          *string `protobuf:"bytes,1,req,name=pattern" json:"pattern,omitempty"`
	PatternFlags     *int32  `protobuf:"varint,2,req,name=pattern_flags" json:"pattern_flags,omitempty"`
	Charset          *string `protobuf:"bytes,3,req,name=charset" json:"charset,omitempty"`
	Engine           *string `protobuf:"bytes,4,opt,name=engine" json:"engine,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *RegexStringComparator) Reset()         { *m = RegexStringComparator{} }
func (m *RegexStringComparator) String() string { return proto1.CompactTextString(m) }
func (*RegexStringComparator) ProtoMessage()    {}

func (m *RegexStringComparator) GetPattern() string {
	if m != nil && m.Pattern != nil {
		return *m.Pattern
	}
	return ""
}

func (m *RegexStringComparator) GetPatternFlags() int32 {
	if m != nil && m.PatternFlags != nil {
		return *m.PatternFlags
	}
	return 0
}

func (m *RegexStringComparator) GetCharset() string {
	if m != nil && m.Charset != nil {
		return *m.Charset
	}
	return ""
}

func (m *RegexStringComparator) GetEngine() string {
	if m != nil && m.Engine != nil {
		return *m.Engine
	}
	return ""
}

type SubstringComparator struct {
	Substr           *string `protobuf:"bytes,1,req,name=substr" json:"substr,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *SubstringComparator) Reset()         { *m = SubstringComparator{} }
func (m *SubstringComparator) String() string { return proto1.CompactTextString(m) }
func (*SubstringComparator) ProtoMessage()    {}

func (m *SubstringComparator) GetSubstr() string {
	if m != nil && m.Substr != nil {
		return *m.Substr
	}
	return ""
}

func init() {
	proto1.RegisterEnum("proto.BitComparator_BitwiseOp", BitComparator_BitwiseOp_name, BitComparator_BitwiseOp_value)
}
