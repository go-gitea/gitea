// Copyright 2015 PingCAP, Inc.
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

package inspectkv

import (
	"io"
	"reflect"

	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/pingcap/tidb/column"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/meta"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/table"
	"github.com/pingcap/tidb/table/tables"
	"github.com/pingcap/tidb/terror"
	"github.com/pingcap/tidb/util"
	"github.com/pingcap/tidb/util/types"
)

// DDLInfo is for DDL information.
type DDLInfo struct {
	SchemaVer   int64
	ReorgHandle int64 // it's only used for DDL information.
	Owner       *model.Owner
	Job         *model.Job
}

// GetDDLInfo returns DDL information.
func GetDDLInfo(txn kv.Transaction) (*DDLInfo, error) {
	var err error
	info := &DDLInfo{}
	t := meta.NewMeta(txn)

	info.Owner, err = t.GetDDLJobOwner()
	if err != nil {
		return nil, errors.Trace(err)
	}
	info.Job, err = t.GetDDLJob(0)
	if err != nil {
		return nil, errors.Trace(err)
	}
	info.SchemaVer, err = t.GetSchemaVersion()
	if err != nil {
		return nil, errors.Trace(err)
	}
	if info.Job == nil {
		return info, nil
	}

	info.ReorgHandle, err = t.GetDDLReorgHandle(info.Job)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return info, nil
}

// GetBgDDLInfo returns background DDL information.
func GetBgDDLInfo(txn kv.Transaction) (*DDLInfo, error) {
	var err error
	info := &DDLInfo{}
	t := meta.NewMeta(txn)

	info.Owner, err = t.GetBgJobOwner()
	if err != nil {
		return nil, errors.Trace(err)
	}
	info.Job, err = t.GetBgJob(0)
	if err != nil {
		return nil, errors.Trace(err)
	}
	info.SchemaVer, err = t.GetSchemaVersion()
	if err != nil {
		return nil, errors.Trace(err)
	}

	return info, nil
}

func nextIndexVals(data []types.Datum) []types.Datum {
	// Add 0x0 to the end of data.
	return append(data, types.Datum{})
}

// RecordData is the record data composed of a handle and values.
type RecordData struct {
	Handle int64
	Values []types.Datum
}

// GetIndexRecordsCount returns the total number of the index records from startVals.
// If startVals = nil, returns the total number of the index records.
func GetIndexRecordsCount(txn kv.Transaction, kvIndex kv.Index, startVals []types.Datum) (int64, error) {
	it, _, err := kvIndex.Seek(txn, startVals)
	if err != nil {
		return 0, errors.Trace(err)
	}
	defer it.Close()

	var cnt int64
	for {
		_, _, err := it.Next()
		if terror.ErrorEqual(err, io.EOF) {
			break
		} else if err != nil {
			return 0, errors.Trace(err)
		}
		cnt++
	}

	return cnt, nil
}

// ScanIndexData scans the index handles and values in a limited number, according to the index information.
// It returns data and the next startVals until it doesn't have data, then returns data is nil and
// the next startVals is the values which can't get data. If startVals = nil and limit = -1,
// it returns the index data of the whole.
func ScanIndexData(txn kv.Transaction, kvIndex kv.Index, startVals []types.Datum, limit int64) (
	[]*RecordData, []types.Datum, error) {
	it, _, err := kvIndex.Seek(txn, startVals)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	defer it.Close()

	var idxRows []*RecordData
	var curVals []types.Datum
	for limit != 0 {
		val, h, err1 := it.Next()
		if terror.ErrorEqual(err1, io.EOF) {
			return idxRows, nextIndexVals(curVals), nil
		} else if err1 != nil {
			return nil, nil, errors.Trace(err1)
		}
		idxRows = append(idxRows, &RecordData{Handle: h, Values: val})
		limit--
		curVals = val
	}

	nextVals, _, err := it.Next()
	if terror.ErrorEqual(err, io.EOF) {
		return idxRows, nextIndexVals(curVals), nil
	} else if err != nil {
		return nil, nil, errors.Trace(err)
	}

	return idxRows, nextVals, nil
}

// CompareIndexData compares index data one by one.
// It returns nil if the data from the index is equal to the data from the table columns,
// otherwise it returns an error with a different set of records.
func CompareIndexData(txn kv.Transaction, t table.Table, idx *column.IndexedCol) error {
	err := checkIndexAndRecord(txn, t, idx)
	if err != nil {
		return errors.Trace(err)
	}

	return checkRecordAndIndex(txn, t, idx)
}

func checkIndexAndRecord(txn kv.Transaction, t table.Table, idx *column.IndexedCol) error {
	kvIndex := kv.NewKVIndex(t.IndexPrefix(), idx.Name.L, idx.ID, idx.Unique)
	it, err := kvIndex.SeekFirst(txn)
	if err != nil {
		return errors.Trace(err)
	}
	defer it.Close()

	cols := make([]*column.Col, len(idx.Columns))
	for i, col := range idx.Columns {
		cols[i] = t.Cols()[col.Offset]
	}

	for {
		vals1, h, err := it.Next()
		if terror.ErrorEqual(err, io.EOF) {
			break
		} else if err != nil {
			return errors.Trace(err)
		}

		vals2, err := rowWithCols(txn, t, h, cols)
		if terror.ErrorEqual(err, kv.ErrNotExist) {
			record := &RecordData{Handle: h, Values: vals1}
			err = errors.Errorf("index:%v != record:%v", record, nil)
		}
		if err != nil {
			return errors.Trace(err)
		}
		if !reflect.DeepEqual(vals1, vals2) {
			record1 := &RecordData{Handle: h, Values: vals1}
			record2 := &RecordData{Handle: h, Values: vals2}
			return errors.Errorf("index:%v != record:%v", record1, record2)
		}
	}

	return nil
}

func checkRecordAndIndex(txn kv.Transaction, t table.Table, idx *column.IndexedCol) error {
	cols := make([]*column.Col, len(idx.Columns))
	for i, col := range idx.Columns {
		cols[i] = t.Cols()[col.Offset]
	}

	startKey := t.RecordKey(0, nil)
	kvIndex := kv.NewKVIndex(t.IndexPrefix(), idx.Name.L, idx.ID, idx.Unique)
	filterFunc := func(h1 int64, vals1 []types.Datum, cols []*column.Col) (bool, error) {
		isExist, h2, err := kvIndex.Exist(txn, vals1, h1)
		if terror.ErrorEqual(err, kv.ErrKeyExists) {
			record1 := &RecordData{Handle: h1, Values: vals1}
			record2 := &RecordData{Handle: h2, Values: vals1}
			return false, errors.Errorf("index:%v != record:%v", record2, record1)
		}
		if err != nil {
			return false, errors.Trace(err)
		}
		if !isExist {
			record := &RecordData{Handle: h1, Values: vals1}
			return false, errors.Errorf("index:%v != record:%v", nil, record)
		}

		return true, nil
	}
	err := iterRecords(txn, t, startKey, cols, filterFunc)

	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

func scanTableData(retriever kv.Retriever, t table.Table, cols []*column.Col, startHandle, limit int64) (
	[]*RecordData, int64, error) {
	var records []*RecordData

	startKey := t.RecordKey(startHandle, nil)
	filterFunc := func(h int64, d []types.Datum, cols []*column.Col) (bool, error) {
		if limit != 0 {
			r := &RecordData{
				Handle: h,
				Values: d,
			}
			records = append(records, r)
			limit--
			return true, nil
		}

		return false, nil
	}
	err := iterRecords(retriever, t, startKey, cols, filterFunc)
	if err != nil {
		return nil, 0, errors.Trace(err)
	}

	if len(records) == 0 {
		return records, startHandle, nil
	}

	nextHandle := records[len(records)-1].Handle + 1

	return records, nextHandle, nil
}

// ScanTableRecord scans table row handles and column values in a limited number.
// It returns data and the next startHandle until it doesn't have data, then returns data is nil and
// the next startHandle is the handle which can't get data. If startHandle = 0 and limit = -1,
// it returns the table data of the whole.
func ScanTableRecord(retriever kv.Retriever, t table.Table, startHandle, limit int64) (
	[]*RecordData, int64, error) {
	return scanTableData(retriever, t, t.Cols(), startHandle, limit)
}

// ScanSnapshotTableRecord scans the ver version of the table data in a limited number.
// It returns data and the next startHandle until it doesn't have data, then returns data is nil and
// the next startHandle is the handle which can't get data. If startHandle = 0 and limit = -1,
// it returns the table data of the whole.
func ScanSnapshotTableRecord(store kv.Storage, ver kv.Version, t table.Table, startHandle, limit int64) (
	[]*RecordData, int64, error) {
	snap, err := store.GetSnapshot(ver)
	if err != nil {
		return nil, 0, errors.Trace(err)
	}
	defer snap.Release()

	records, nextHandle, err := ScanTableRecord(snap, t, startHandle, limit)

	return records, nextHandle, errors.Trace(err)
}

// CompareTableRecord compares data and the corresponding table data one by one.
// It returns nil if data is equal to the data that scans from table, otherwise
// it returns an error with a different set of records. If exact is false, only compares handle.
func CompareTableRecord(txn kv.Transaction, t table.Table, data []*RecordData, exact bool) error {
	m := make(map[int64][]types.Datum, len(data))
	for _, r := range data {
		if _, ok := m[r.Handle]; ok {
			return errors.Errorf("handle:%d is repeated in data", r.Handle)
		}
		m[r.Handle] = r.Values
	}

	startKey := t.RecordKey(0, nil)
	filterFunc := func(h int64, vals []types.Datum, cols []*column.Col) (bool, error) {
		vals2, ok := m[h]
		if !ok {
			record := &RecordData{Handle: h, Values: vals}
			return false, errors.Errorf("data:%v != record:%v", nil, record)
		}
		if !exact {
			delete(m, h)
			return true, nil
		}

		if !reflect.DeepEqual(vals, vals2) {
			record1 := &RecordData{Handle: h, Values: vals2}
			record2 := &RecordData{Handle: h, Values: vals}
			return false, errors.Errorf("data:%v != record:%v", record1, record2)
		}

		delete(m, h)

		return true, nil
	}
	err := iterRecords(txn, t, startKey, t.Cols(), filterFunc)
	if err != nil {
		return errors.Trace(err)
	}

	for h, vals := range m {
		record := &RecordData{Handle: h, Values: vals}
		return errors.Errorf("data:%v != record:%v", record, nil)
	}

	return nil
}

// GetTableRecordsCount returns the total number of table records from startHandle.
// If startHandle = 0, returns the total number of table records.
func GetTableRecordsCount(txn kv.Transaction, t table.Table, startHandle int64) (int64, error) {
	startKey := t.RecordKey(startHandle, nil)
	it, err := txn.Seek(startKey)
	if err != nil {
		return 0, errors.Trace(err)
	}

	var cnt int64
	prefix := t.RecordPrefix()
	for it.Valid() && it.Key().HasPrefix(prefix) {
		handle, err := tables.DecodeRecordKeyHandle(it.Key())
		if err != nil {
			return 0, errors.Trace(err)
		}

		it.Close()
		rk := t.RecordKey(handle+1, nil)
		it, err = txn.Seek(rk)
		if err != nil {
			return 0, errors.Trace(err)
		}

		cnt++
	}

	it.Close()

	return cnt, nil
}

func rowWithCols(txn kv.Retriever, t table.Table, h int64, cols []*column.Col) ([]types.Datum, error) {
	v := make([]types.Datum, len(cols))
	for i, col := range cols {
		if col.State != model.StatePublic {
			return nil, errors.Errorf("Cannot use none public column - %v", cols)
		}
		if col.IsPKHandleColumn(t.Meta()) {
			v[i].SetInt64(h)
			continue
		}

		k := t.RecordKey(h, col)
		data, err := txn.Get(k)
		if err != nil {
			return nil, errors.Trace(err)
		}

		val, err := tables.DecodeValue(data, &col.FieldType)
		if err != nil {
			return nil, errors.Trace(err)
		}
		v[i] = val
	}
	return v, nil
}

func iterRecords(retriever kv.Retriever, t table.Table, startKey kv.Key, cols []*column.Col,
	fn table.RecordIterFunc) error {
	it, err := retriever.Seek(startKey)
	if err != nil {
		return errors.Trace(err)
	}
	defer it.Close()

	if !it.Valid() {
		return nil
	}

	log.Debugf("startKey:%q, key:%q, value:%q", startKey, it.Key(), it.Value())

	prefix := t.RecordPrefix()
	for it.Valid() && it.Key().HasPrefix(prefix) {
		// first kv pair is row lock information.
		// TODO: check valid lock
		// get row handle
		handle, err := tables.DecodeRecordKeyHandle(it.Key())
		if err != nil {
			return errors.Trace(err)
		}

		data, err := rowWithCols(retriever, t, handle, cols)
		if err != nil {
			return errors.Trace(err)
		}
		more, err := fn(handle, data, cols)
		if !more || err != nil {
			return errors.Trace(err)
		}

		rk := t.RecordKey(handle, nil)
		err = kv.NextUntil(it, util.RowKeyPrefixFilter(rk))
		if err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}
