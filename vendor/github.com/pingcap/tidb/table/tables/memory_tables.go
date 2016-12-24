// Copyright 2016 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package tables

import (
	"sync"

	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/petar/GoLLRB/llrb"
	"github.com/pingcap/tidb/column"
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/meta/autoid"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/table"
	"github.com/pingcap/tidb/util/types"
)

var (
	errRowNotFound = errors.New("Can not find the row")
)

type itemKey int64

type itemPair struct {
	handle itemKey
	data   []types.Datum
}

func (r *itemPair) Less(item llrb.Item) bool {
	switch x := item.(type) {
	case itemKey:
		return r.handle < x
	case *itemPair:
		return r.handle < x.handle
	}
	log.Errorf("invalid type %T", item)
	return true
}

func (k itemKey) Less(item llrb.Item) bool {
	switch x := item.(type) {
	case itemKey:
		return k < x
	case *itemPair:
		return k < x.handle
	}
	log.Errorf("invalid type %T", item)
	return true
}

// MemoryTable implements table.Table interface.
type MemoryTable struct {
	ID          int64
	Name        model.CIStr
	Columns     []*column.Col
	pkHandleCol *column.Col

	recordPrefix kv.Key
	alloc        autoid.Allocator
	meta         *model.TableInfo

	tree *llrb.LLRB
	mu   sync.RWMutex
}

// MemoryTableFromMeta creates a Table instance from model.TableInfo.
func MemoryTableFromMeta(alloc autoid.Allocator, tblInfo *model.TableInfo) (table.Table, error) {
	columns := make([]*column.Col, 0, len(tblInfo.Columns))
	var pkHandleColumn *column.Col
	for _, colInfo := range tblInfo.Columns {
		col := &column.Col{ColumnInfo: *colInfo}
		columns = append(columns, col)
		if col.IsPKHandleColumn(tblInfo) {
			pkHandleColumn = col
		}
	}
	t := newMemoryTable(tblInfo.ID, tblInfo.Name.O, columns, alloc)
	t.pkHandleCol = pkHandleColumn
	t.meta = tblInfo
	return t, nil
}

// newMemoryTable constructs a MemoryTable instance.
func newMemoryTable(tableID int64, tableName string, cols []*column.Col, alloc autoid.Allocator) *MemoryTable {
	name := model.NewCIStr(tableName)
	t := &MemoryTable{
		ID:           tableID,
		Name:         name,
		alloc:        alloc,
		Columns:      cols,
		recordPrefix: genTableRecordPrefix(tableID),
		tree:         llrb.New(),
	}
	return t
}

// Seek seeks the handle
func (t *MemoryTable) Seek(ctx context.Context, handle int64) (int64, bool, error) {
	var found bool
	var result int64
	t.mu.RLock()
	t.tree.AscendGreaterOrEqual(itemKey(handle), func(item llrb.Item) bool {
		found = true
		result = int64(item.(*itemPair).handle)
		return false
	})
	t.mu.RUnlock()
	return result, found, nil
}

// Indices implements table.Table Indices interface.
func (t *MemoryTable) Indices() []*column.IndexedCol {
	return nil
}

// Meta implements table.Table Meta interface.
func (t *MemoryTable) Meta() *model.TableInfo {
	return t.meta
}

// Cols implements table.Table Cols interface.
func (t *MemoryTable) Cols() []*column.Col {
	return t.Columns
}

// RecordPrefix implements table.Table RecordPrefix interface.
func (t *MemoryTable) RecordPrefix() kv.Key {
	return t.recordPrefix
}

// IndexPrefix implements table.Table IndexPrefix interface.
func (t *MemoryTable) IndexPrefix() kv.Key {
	return nil
}

// RecordKey implements table.Table RecordKey interface.
func (t *MemoryTable) RecordKey(h int64, col *column.Col) kv.Key {
	colID := int64(0)
	if col != nil {
		colID = col.ID
	}
	return encodeRecordKey(t.recordPrefix, h, colID)
}

// FirstKey implements table.Table FirstKey interface.
func (t *MemoryTable) FirstKey() kv.Key {
	return t.RecordKey(0, nil)
}

// Truncate implements table.Table Truncate interface.
func (t *MemoryTable) Truncate(ctx context.Context) error {
	t.tree = llrb.New()
	return nil
}

// UpdateRecord implements table.Table UpdateRecord interface.
func (t *MemoryTable) UpdateRecord(ctx context.Context, h int64, oldData []types.Datum, newData []types.Datum, touched map[int]bool) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	item := t.tree.Get(itemKey(h))
	if item == nil {
		return errRowNotFound
	}
	pair := item.(*itemPair)
	pair.data = newData
	return nil
}

// AddRecord implements table.Table AddRecord interface.
func (t *MemoryTable) AddRecord(ctx context.Context, r []types.Datum) (recordID int64, err error) {
	if t.pkHandleCol != nil {
		recordID, err = types.ToInt64(r[t.pkHandleCol.Offset].GetValue())
		if err != nil {
			return 0, errors.Trace(err)
		}
	} else {
		recordID, err = t.alloc.Alloc(t.ID)
		if err != nil {
			return 0, errors.Trace(err)
		}
	}
	item := &itemPair{
		handle: itemKey(recordID),
		data:   r,
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.tree.Get(itemKey(recordID)) != nil {
		return 0, kv.ErrKeyExists
	}
	t.tree.ReplaceOrInsert(item)
	return
}

// RowWithCols implements table.Table RowWithCols interface.
func (t *MemoryTable) RowWithCols(ctx context.Context, h int64, cols []*column.Col) ([]types.Datum, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	item := t.tree.Get(itemKey(h))
	if item == nil {
		return nil, errRowNotFound
	}
	row := item.(*itemPair).data
	v := make([]types.Datum, len(cols))
	for i, col := range cols {
		v[i] = row[col.Offset]
	}
	return v, nil
}

// Row implements table.Table Row interface.
func (t *MemoryTable) Row(ctx context.Context, h int64) ([]types.Datum, error) {
	r, err := t.RowWithCols(nil, h, t.Cols())
	if err != nil {
		return nil, errors.Trace(err)
	}
	return r, nil
}

// LockRow implements table.Table LockRow interface.
func (t *MemoryTable) LockRow(ctx context.Context, h int64, forRead bool) error {
	return nil
}

// RemoveRecord implements table.Table RemoveRecord interface.
func (t *MemoryTable) RemoveRecord(ctx context.Context, h int64, r []types.Datum) error {
	t.mu.Lock()
	t.tree.Delete(itemKey(h))
	t.mu.Unlock()
	return nil
}

// AllocAutoID implements table.Table AllocAutoID interface.
func (t *MemoryTable) AllocAutoID() (int64, error) {
	return t.alloc.Alloc(t.ID)
}

// RebaseAutoID implements table.Table RebaseAutoID interface.
func (t *MemoryTable) RebaseAutoID(newBase int64, isSetStep bool) error {
	return t.alloc.Rebase(t.ID, newBase, isSetStep)
}

// IterRecords implements table.Table IterRecords interface.
func (t *MemoryTable) IterRecords(ctx context.Context, startKey kv.Key, cols []*column.Col,
	fn table.RecordIterFunc) error {
	return nil
}
