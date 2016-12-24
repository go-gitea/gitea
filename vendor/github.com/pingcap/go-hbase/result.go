package hbase

import (
	"fmt"

	"github.com/pingcap/go-hbase/proto"
)

type Kv struct {
	Row   []byte
	Ts    uint64
	Value []byte
	// history results
	Values map[uint64][]byte
	Column
}

func (kv *Kv) String() string {
	if kv == nil {
		return "<nil>"
	}
	return fmt.Sprintf("Kv(%+v)", *kv)
}

type ResultRow struct {
	Row           []byte
	Columns       map[string]*Kv
	SortedColumns []*Kv
}

func (r *ResultRow) String() string {
	if r == nil {
		return "<nil>"
	}
	return fmt.Sprintf("ResultRow(%+v)", *r)
}

func NewResultRow(result *proto.Result) *ResultRow {
	// empty response
	if len(result.GetCell()) == 0 {
		return nil
	}
	res := &ResultRow{}
	res.Columns = make(map[string]*Kv)
	res.SortedColumns = make([]*Kv, 0)

	for _, cell := range result.GetCell() {
		res.Row = cell.GetRow()

		col := &Kv{
			Row: res.Row,
			Column: Column{
				Family: cell.GetFamily(),
				Qual:   cell.GetQualifier(),
			},
			Value: cell.GetValue(),
			Ts:    cell.GetTimestamp(),
		}

		colName := string(col.Column.Family) + ":" + string(col.Column.Qual)

		if v, exists := res.Columns[colName]; exists {
			// renew the same cf result
			if col.Ts > v.Ts {
				v.Value = col.Value
				v.Ts = col.Ts
			}
			v.Values[col.Ts] = col.Value
		} else {
			col.Values = map[uint64][]byte{col.Ts: col.Value}
			res.Columns[colName] = col
			res.SortedColumns = append(res.SortedColumns, col)
		}
	}
	return res
}
