package themis

import (
	"fmt"
	"sort"

	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/pingcap/go-hbase"
	"github.com/pingcap/go-hbase/proto"
)

type mutationValuePair struct {
	typ   hbase.Type
	value []byte
}

func (mp *mutationValuePair) String() string {
	return fmt.Sprintf("type: %d value: %s", mp.typ, mp.value)
}

type columnMutation struct {
	*hbase.Column
	*mutationValuePair
}

func getEntriesFromDel(p *hbase.Delete) ([]*columnMutation, error) {
	errMsg := "must set at least one column for themis delete"
	if len(p.FamilyQuals) == 0 {
		return nil, errors.New(errMsg)
	}

	var ret []*columnMutation
	for f, _ := range p.Families {
		quilifiers := p.FamilyQuals[f]
		if len(quilifiers) == 0 {
			return nil, errors.New(errMsg)
		}
		for q, _ := range quilifiers {
			mutation := &columnMutation{
				Column: &hbase.Column{
					Family: []byte(f),
					Qual:   []byte(q),
				},
				mutationValuePair: &mutationValuePair{
					typ: hbase.TypeDeleteColumn,
				},
			}
			ret = append(ret, mutation)
		}
	}
	return ret, nil
}

func getEntriesFromPut(p *hbase.Put) []*columnMutation {
	var ret []*columnMutation
	for i, f := range p.Families {
		qualifiers := p.Qualifiers[i]
		for j, q := range qualifiers {
			mutation := &columnMutation{
				Column: &hbase.Column{
					Family: f,
					Qual:   q,
				},
				mutationValuePair: &mutationValuePair{
					typ:   hbase.TypePut,
					value: p.Values[i][j],
				},
			}
			ret = append(ret, mutation)
		}
	}
	return ret
}

func (cm *columnMutation) toCell() *proto.Cell {
	ret := &proto.Cell{
		Family:    cm.Family,
		Qualifier: cm.Qual,
		Value:     cm.value,
	}
	if cm.typ == hbase.TypePut { // put
		ret.CellType = proto.CellType_PUT.Enum()
	} else if cm.typ == hbase.TypeMinimum { // onlyLock
		ret.CellType = proto.CellType_MINIMUM.Enum()
	} else { // delete, themis delete API only support delete column
		ret.CellType = proto.CellType_DELETE_COLUMN.Enum()
	}
	return ret
}

type rowMutation struct {
	tbl []byte
	row []byte
	// mutations := { 'cf:col' => mutationValuePair }
	mutations map[string]*mutationValuePair
}

func (r *rowMutation) getColumns() []hbase.Column {
	var ret []hbase.Column
	for k, _ := range r.mutations {
		c := &hbase.Column{}
		// TODO: handle error, now just ignore
		if err := c.ParseFromString(k); err != nil {
			log.Warnf("parse from string error, column: %s, mutation: %s, error: %v", c, k, err)
		}
		ret = append(ret, *c)
	}
	return ret
}

func (r *rowMutation) getSize() int {
	return len(r.mutations)
}

func (r *rowMutation) getType(c hbase.Column) hbase.Type {
	p, ok := r.mutations[c.String()]
	if !ok {
		return hbase.TypeMinimum
	}
	return p.typ
}

func newRowMutation(tbl, row []byte) *rowMutation {
	return &rowMutation{
		tbl:       tbl,
		row:       row,
		mutations: map[string]*mutationValuePair{},
	}
}

func (r *rowMutation) addMutation(c *hbase.Column, typ hbase.Type, val []byte, onlyLock bool) {
	// 3 scene: put, delete, onlyLock
	// if it is onlyLock scene, then has not data modify, when has contained the qualifier, can't replace exist value,
	// becuase put or delete operation has add mutation
	if onlyLock && r.mutations[c.String()] != nil {
		return
	}

	r.mutations[c.String()] = &mutationValuePair{
		typ:   typ,
		value: val,
	}
}

func (r *rowMutation) mutationList(withValue bool) []*columnMutation {
	var ret []*columnMutation
	var keys []string
	for k, _ := range r.mutations {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := &mutationValuePair{
			typ: r.mutations[k].typ,
		}
		if withValue {
			v.value = r.mutations[k].value
		}
		c := &hbase.Column{}
		// TODO: handle error, now just ignore
		if err := c.ParseFromString(k); err != nil {
			log.Warnf("parse from string error, column: %s, mutation: %s, error: %v", c, k, err)
		}
		ret = append(ret, &columnMutation{
			Column:            c,
			mutationValuePair: v,
		})
	}
	return ret
}

type columnMutationCache struct {
	// mutations => {table => { rowKey => row mutations } }
	mutations map[string]map[string]*rowMutation
}

func newColumnMutationCache() *columnMutationCache {
	return &columnMutationCache{
		mutations: map[string]map[string]*rowMutation{},
	}
}

func (c *columnMutationCache) addMutation(tbl []byte, row []byte, col *hbase.Column, t hbase.Type, v []byte, onlyLock bool) {
	tblRowMutations, ok := c.mutations[string(tbl)]
	if !ok {
		// create table mutation map
		tblRowMutations = map[string]*rowMutation{}
		c.mutations[string(tbl)] = tblRowMutations
	}

	rowMutations, ok := tblRowMutations[string(row)]
	if !ok {
		// create row mutation map
		rowMutations = newRowMutation(tbl, row)
		tblRowMutations[string(row)] = rowMutations
	}
	rowMutations.addMutation(col, t, v, onlyLock)
}

func (c *columnMutationCache) getMutation(cc *hbase.ColumnCoordinate) *mutationValuePair {
	t, ok := c.mutations[string(cc.Table)]
	if !ok {
		return nil
	}
	rowMutation, ok := t[string(cc.Row)]
	if !ok {
		return nil
	}
	p, ok := rowMutation.mutations[cc.GetColumn().String()]
	if !ok {
		return nil
	}
	return p
}

func (c *columnMutationCache) getRowCount() int {
	ret := 0
	for _, v := range c.mutations {
		ret += len(v)
	}
	return ret
}

func (c *columnMutationCache) getMutationCount() int {
	ret := 0
	for _, v := range c.mutations {
		for _, vv := range v {
			ret += len(vv.mutationList(false))
		}
	}
	return ret
}
