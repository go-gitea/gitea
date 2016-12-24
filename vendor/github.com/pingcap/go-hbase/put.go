package hbase

import (
	"bytes"
	"math"

	pb "github.com/golang/protobuf/proto"
	"github.com/pingcap/go-hbase/proto"
)

type Put struct {
	Row        []byte
	Families   [][]byte
	Qualifiers [][][]byte
	Values     [][][]byte
	Timestamp  uint64
}

func NewPut(row []byte) *Put {
	return &Put{
		Row:        row,
		Families:   make([][]byte, 0),
		Qualifiers: make([][][]byte, 0),
		Values:     make([][][]byte, 0),
	}
}

func (p *Put) GetRow() []byte {
	return p.Row
}

func (p *Put) AddValue(family, qual, value []byte) *Put {
	pos := p.posOfFamily(family)
	if pos == -1 {
		p.Families = append(p.Families, family)
		p.Qualifiers = append(p.Qualifiers, make([][]byte, 0))
		p.Values = append(p.Values, make([][]byte, 0))

		pos = p.posOfFamily(family)
	}

	p.Qualifiers[pos] = append(p.Qualifiers[pos], qual)
	p.Values[pos] = append(p.Values[pos], value)
	return p
}

func (p *Put) AddStringValue(family, column, value string) *Put {
	return p.AddValue([]byte(family), []byte(column), []byte(value))
}

func (p *Put) AddTimestamp(ts uint64) *Put {
	if ts == 0 {
		p.Timestamp = math.MaxInt64
	} else {
		p.Timestamp = ts
	}
	return p
}

func (p *Put) posOfFamily(family []byte) int {
	for p, v := range p.Families {
		if bytes.Equal(family, v) {
			return p
		}
	}
	return -1
}

func (p *Put) ToProto() pb.Message {
	put := &proto.MutationProto{
		Row:        p.Row,
		MutateType: proto.MutationProto_PUT.Enum(),
	}

	for i, family := range p.Families {
		cv := &proto.MutationProto_ColumnValue{
			Family: family,
		}

		for j := range p.Qualifiers[i] {
			cv.QualifierValue = append(cv.QualifierValue, &proto.MutationProto_ColumnValue_QualifierValue{
				Qualifier: p.Qualifiers[i][j],
				Value:     p.Values[i][j],
				Timestamp: pb.Uint64(p.Timestamp),
			})
		}

		put.ColumnValue = append(put.ColumnValue, cv)
	}

	return put
}
