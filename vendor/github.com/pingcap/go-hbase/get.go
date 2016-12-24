package hbase

import (
	"strings"

	pb "github.com/golang/protobuf/proto"
	"github.com/juju/errors"
	"github.com/pingcap/go-hbase/proto"
)

type Get struct {
	Row         []byte
	Families    set
	FamilyQuals map[string]set
	Versions    int32
	TsRangeFrom uint64
	TsRangeTo   uint64
}

func NewGet(row []byte) *Get {
	return &Get{
		Row:         append([]byte(nil), row...),
		Families:    newSet(),
		FamilyQuals: make(map[string]set),
		Versions:    1,
	}
}

func (g *Get) GetRow() []byte {
	return g.Row
}

func (g *Get) AddString(famqual string) error {
	parts := strings.Split(famqual, ":")

	if len(parts) > 2 {
		return errors.Errorf("Too many colons were found in the family:qualifier string. '%s'", famqual)
	} else if len(parts) == 2 {
		g.AddStringColumn(parts[0], parts[1])
	} else {
		g.AddStringFamily(famqual)
	}

	return nil
}

func (g *Get) AddColumn(family, qual []byte) *Get {
	g.AddFamily(family)
	g.FamilyQuals[string(family)].add(string(qual))
	return g
}

func (g *Get) AddStringColumn(family, qual string) *Get {
	return g.AddColumn([]byte(family), []byte(qual))
}

func (g *Get) AddFamily(family []byte) *Get {
	g.Families.add(string(family))
	if _, ok := g.FamilyQuals[string(family)]; !ok {
		g.FamilyQuals[string(family)] = newSet()
	}
	return g
}

func (g *Get) AddStringFamily(family string) *Get {
	return g.AddFamily([]byte(family))
}

func (g *Get) AddTimeRange(from uint64, to uint64) *Get {
	g.TsRangeFrom = from
	g.TsRangeTo = to
	return g
}

func (g *Get) SetMaxVersion(maxVersion int32) *Get {
	g.Versions = maxVersion
	return g
}

func (g *Get) ToProto() pb.Message {
	get := &proto.Get{
		Row: g.Row,
	}

	if g.TsRangeFrom != 0 && g.TsRangeTo != 0 && g.TsRangeFrom <= g.TsRangeTo {
		get.TimeRange = &proto.TimeRange{
			From: pb.Uint64(g.TsRangeFrom),
			To:   pb.Uint64(g.TsRangeTo),
		}
	}

	for v := range g.Families {
		col := &proto.Column{
			Family: []byte(v),
		}
		var quals [][]byte
		for qual := range g.FamilyQuals[v] {
			quals = append(quals, []byte(qual))
		}
		col.Qualifier = quals
		get.Column = append(get.Column, col)
	}
	get.MaxVersions = pb.Uint32(uint32(g.Versions))
	return get
}
