// Copyright 2013 The ql Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSES/QL-LICENSE file.

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

package tables

import (
	"strings"
	"time"

	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/pingcap/tidb/column"
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/evaluator"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/meta/autoid"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/sessionctx/variable"
	"github.com/pingcap/tidb/table"
	"github.com/pingcap/tidb/terror"
	"github.com/pingcap/tidb/util"
	"github.com/pingcap/tidb/util/codec"
	"github.com/pingcap/tidb/util/types"
)

// TablePrefix is the prefix for table record and index key.
var TablePrefix = []byte{'t'}

// Table implements table.Table interface.
type Table struct {
	ID      int64
	Name    model.CIStr
	Columns []*column.Col

	publicColumns   []*column.Col
	writableColumns []*column.Col
	indices         []*column.IndexedCol
	recordPrefix    kv.Key
	indexPrefix     kv.Key
	alloc           autoid.Allocator
	meta            *model.TableInfo
}

// TableFromMeta creates a Table instance from model.TableInfo.
func TableFromMeta(alloc autoid.Allocator, tblInfo *model.TableInfo) (table.Table, error) {
	if tblInfo.State == model.StateNone {
		return nil, errors.Errorf("table %s can't be in none state", tblInfo.Name)
	}

	columns := make([]*column.Col, 0, len(tblInfo.Columns))
	for _, colInfo := range tblInfo.Columns {
		if colInfo.State == model.StateNone {
			return nil, errors.Errorf("column %s can't be in none state", colInfo.Name)
		}

		col := &column.Col{ColumnInfo: *colInfo}
		columns = append(columns, col)
	}

	t := newTable(tblInfo.ID, columns, alloc)

	for _, idxInfo := range tblInfo.Indices {
		if idxInfo.State == model.StateNone {
			return nil, errors.Errorf("index %s can't be in none state", idxInfo.Name)
		}

		idx := &column.IndexedCol{
			IndexInfo: *idxInfo,
		}

		idx.X = kv.NewKVIndex(t.IndexPrefix(), idxInfo.Name.L, idxInfo.ID, idxInfo.Unique)

		t.indices = append(t.indices, idx)
	}
	t.meta = tblInfo
	return t, nil
}

// newTable constructs a Table instance.
func newTable(tableID int64, cols []*column.Col, alloc autoid.Allocator) *Table {
	t := &Table{
		ID:           tableID,
		recordPrefix: genTableRecordPrefix(tableID),
		indexPrefix:  genTableIndexPrefix(tableID),
		alloc:        alloc,
		Columns:      cols,
	}

	t.publicColumns = t.Cols()
	t.writableColumns = t.writableCols()
	return t
}

// Indices implements table.Table Indices interface.
func (t *Table) Indices() []*column.IndexedCol {
	return t.indices
}

// Meta implements table.Table Meta interface.
func (t *Table) Meta() *model.TableInfo {
	return t.meta
}

// Cols implements table.Table Cols interface.
func (t *Table) Cols() []*column.Col {
	if len(t.publicColumns) > 0 {
		return t.publicColumns
	}

	t.publicColumns = make([]*column.Col, 0, len(t.Columns))
	for _, col := range t.Columns {
		if col.State == model.StatePublic {
			t.publicColumns = append(t.publicColumns, col)
		}
	}

	return t.publicColumns
}

func (t *Table) writableCols() []*column.Col {
	if len(t.writableColumns) > 0 {
		return t.writableColumns
	}

	t.writableColumns = make([]*column.Col, 0, len(t.Columns))
	for _, col := range t.Columns {
		if col.State == model.StateDeleteOnly || col.State == model.StateDeleteReorganization {
			continue
		}

		t.writableColumns = append(t.writableColumns, col)
	}

	return t.writableColumns
}

// RecordPrefix implements table.Table RecordPrefix interface.
func (t *Table) RecordPrefix() kv.Key {
	return t.recordPrefix
}

// IndexPrefix implements table.Table IndexPrefix interface.
func (t *Table) IndexPrefix() kv.Key {
	return t.indexPrefix
}

// RecordKey implements table.Table RecordKey interface.
func (t *Table) RecordKey(h int64, col *column.Col) kv.Key {
	colID := int64(0)
	if col != nil {
		colID = col.ID
	}
	return encodeRecordKey(t.recordPrefix, h, colID)
}

// FirstKey implements table.Table FirstKey interface.
func (t *Table) FirstKey() kv.Key {
	return t.RecordKey(0, nil)
}

// Truncate implements table.Table Truncate interface.
func (t *Table) Truncate(ctx context.Context) error {
	txn, err := ctx.GetTxn(false)
	if err != nil {
		return errors.Trace(err)
	}
	err = util.DelKeyWithPrefix(txn, t.RecordPrefix())
	if err != nil {
		return errors.Trace(err)
	}
	return util.DelKeyWithPrefix(txn, t.IndexPrefix())
}

// UpdateRecord implements table.Table UpdateRecord interface.
func (t *Table) UpdateRecord(ctx context.Context, h int64, oldData []types.Datum, newData []types.Datum, touched map[int]bool) error {
	// We should check whether this table has on update column which state is write only.
	currentData := make([]types.Datum, len(t.writableCols()))
	copy(currentData, newData)

	// If they are not set, and other data are changed, they will be updated by current timestamp too.
	err := t.setOnUpdateData(ctx, touched, currentData)
	if err != nil {
		return errors.Trace(err)
	}

	txn, err := ctx.GetTxn(false)
	if err != nil {
		return errors.Trace(err)
	}

	bs := kv.NewBufferStore(txn)
	defer bs.Release()

	// set new value
	if err = t.setNewData(bs, h, touched, currentData); err != nil {
		return errors.Trace(err)
	}

	// rebuild index
	if err = t.rebuildIndices(bs, h, touched, oldData, currentData); err != nil {
		return errors.Trace(err)
	}

	err = bs.SaveTo(txn)
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (t *Table) setOnUpdateData(ctx context.Context, touched map[int]bool, data []types.Datum) error {
	ucols := column.FindOnUpdateCols(t.writableCols())
	for _, col := range ucols {
		if !touched[col.Offset] {
			value, err := evaluator.GetTimeValue(ctx, evaluator.CurrentTimestamp, col.Tp, col.Decimal)
			if err != nil {
				return errors.Trace(err)
			}

			data[col.Offset] = types.NewDatum(value)
			touched[col.Offset] = true
		}
	}
	return nil
}
func (t *Table) setNewData(rm kv.RetrieverMutator, h int64, touched map[int]bool, data []types.Datum) error {
	for _, col := range t.Cols() {
		if !touched[col.Offset] {
			continue
		}

		k := t.RecordKey(h, col)
		if err := SetColValue(rm, k, data[col.Offset]); err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}

func (t *Table) rebuildIndices(rm kv.RetrieverMutator, h int64, touched map[int]bool, oldData []types.Datum, newData []types.Datum) error {
	for _, idx := range t.Indices() {
		idxTouched := false
		for _, ic := range idx.Columns {
			if touched[ic.Offset] {
				idxTouched = true
				break
			}
		}
		if !idxTouched {
			continue
		}

		oldVs, err := idx.FetchValues(oldData)
		if err != nil {
			return errors.Trace(err)
		}

		if t.removeRowIndex(rm, h, oldVs, idx); err != nil {
			return errors.Trace(err)
		}

		newVs, err := idx.FetchValues(newData)
		if err != nil {
			return errors.Trace(err)
		}

		if err := t.buildIndexForRow(rm, h, newVs, idx); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// AddRecord implements table.Table AddRecord interface.
func (t *Table) AddRecord(ctx context.Context, r []types.Datum) (recordID int64, err error) {
	var hasRecordID bool
	for _, col := range t.Cols() {
		if col.IsPKHandleColumn(t.meta) {
			recordID = r[col.Offset].GetInt64()
			hasRecordID = true
			break
		}
	}
	if !hasRecordID {
		recordID, err = t.alloc.Alloc(t.ID)
		if err != nil {
			return 0, errors.Trace(err)
		}
	}
	txn, err := ctx.GetTxn(false)
	if err != nil {
		return 0, errors.Trace(err)
	}
	bs := kv.NewBufferStore(txn)
	defer bs.Release()
	// Insert new entries into indices.
	h, err := t.addIndices(ctx, recordID, r, bs)
	if err != nil {
		return h, errors.Trace(err)
	}

	if err = t.LockRow(ctx, recordID, false); err != nil {
		return 0, errors.Trace(err)
	}
	// Set public and write only column value.
	for _, col := range t.writableCols() {
		if col.IsPKHandleColumn(t.meta) {
			continue
		}
		var value types.Datum
		if col.State == model.StateWriteOnly || col.State == model.StateWriteReorganization {
			// if col is in write only or write reorganization state, we must add it with its default value.
			value, _, err = table.GetColDefaultValue(ctx, &col.ColumnInfo)
			if err != nil {
				return 0, errors.Trace(err)
			}
			value, err = value.ConvertTo(&col.FieldType)
			if err != nil {
				return 0, errors.Trace(err)
			}
		} else {
			value = r[col.Offset]
		}

		key := t.RecordKey(recordID, col)
		err = SetColValue(txn, key, value)
		if err != nil {
			return 0, errors.Trace(err)
		}
	}
	if err = bs.SaveTo(txn); err != nil {
		return 0, errors.Trace(err)
	}

	variable.GetSessionVars(ctx).AddAffectedRows(1)
	return recordID, nil
}

// Generate index content string representation.
func (t *Table) genIndexKeyStr(colVals []types.Datum) (string, error) {
	// Pass pre-composed error to txn.
	strVals := make([]string, 0, len(colVals))
	for _, cv := range colVals {
		cvs := "NULL"
		var err error
		if cv.Kind() != types.KindNull {
			cvs, err = types.ToString(cv.GetValue())
			if err != nil {
				return "", errors.Trace(err)
			}
		}
		strVals = append(strVals, cvs)
	}
	return strings.Join(strVals, "-"), nil
}

// Add data into indices.
func (t *Table) addIndices(ctx context.Context, recordID int64, r []types.Datum, bs *kv.BufferStore) (int64, error) {
	txn, err := ctx.GetTxn(false)
	if err != nil {
		return 0, errors.Trace(err)
	}
	// Clean up lazy check error environment
	defer txn.DelOption(kv.PresumeKeyNotExistsError)
	if t.meta.PKIsHandle {
		// Check key exists.
		recordKey := t.RecordKey(recordID, nil)
		e := kv.ErrKeyExists.Gen("Duplicate entry '%d' for key 'PRIMARY'", recordID)
		txn.SetOption(kv.PresumeKeyNotExistsError, e)
		_, err = txn.Get(recordKey)
		if err == nil {
			return recordID, errors.Trace(e)
		} else if !terror.ErrorEqual(err, kv.ErrNotExist) {
			return 0, errors.Trace(err)
		}
		txn.DelOption(kv.PresumeKeyNotExistsError)
	}

	for _, v := range t.indices {
		if v == nil || v.State == model.StateDeleteOnly || v.State == model.StateDeleteReorganization {
			// if index is in delete only or delete reorganization state, we can't add it.
			continue
		}
		colVals, _ := v.FetchValues(r)
		var dupKeyErr error
		if v.Unique || v.Primary {
			entryKey, err1 := t.genIndexKeyStr(colVals)
			if err1 != nil {
				return 0, errors.Trace(err1)
			}
			dupKeyErr = kv.ErrKeyExists.Gen("Duplicate entry '%s' for key '%s'", entryKey, v.Name)
			txn.SetOption(kv.PresumeKeyNotExistsError, dupKeyErr)
		}
		if err = v.X.Create(bs, colVals, recordID); err != nil {
			if terror.ErrorEqual(err, kv.ErrKeyExists) {
				// Get the duplicate row handle
				// For insert on duplicate syntax, we should update the row
				iter, _, err1 := v.X.Seek(bs, colVals)
				if err1 != nil {
					return 0, errors.Trace(err1)
				}
				_, h, err1 := iter.Next()
				if err1 != nil {
					return 0, errors.Trace(err1)
				}
				return h, errors.Trace(dupKeyErr)
			}
			return 0, errors.Trace(err)
		}
		txn.DelOption(kv.PresumeKeyNotExistsError)
	}
	return 0, nil
}

// RowWithCols implements table.Table RowWithCols interface.
func (t *Table) RowWithCols(ctx context.Context, h int64, cols []*column.Col) ([]types.Datum, error) {
	txn, err := ctx.GetTxn(false)
	if err != nil {
		return nil, errors.Trace(err)
	}
	v := make([]types.Datum, len(cols))
	for i, col := range cols {
		if col.State != model.StatePublic {
			return nil, errors.Errorf("Cannot use none public column - %v", cols)
		}
		if col.IsPKHandleColumn(t.meta) {
			v[i].SetInt64(h)
			continue
		}

		k := t.RecordKey(h, col)
		data, err := txn.Get(k)
		if err != nil {
			return nil, errors.Trace(err)
		}

		v[i], err = DecodeValue(data, &col.FieldType)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	return v, nil
}

// Row implements table.Table Row interface.
func (t *Table) Row(ctx context.Context, h int64) ([]types.Datum, error) {
	// TODO: we only interested in mentioned cols
	r, err := t.RowWithCols(ctx, h, t.Cols())
	if err != nil {
		return nil, errors.Trace(err)
	}
	return r, nil
}

// LockRow implements table.Table LockRow interface.
func (t *Table) LockRow(ctx context.Context, h int64, forRead bool) error {
	txn, err := ctx.GetTxn(false)
	if err != nil {
		return errors.Trace(err)
	}
	// Get row lock key
	lockKey := t.RecordKey(h, nil)
	if forRead {
		err = txn.LockKeys(lockKey)
	} else {
		// set row lock key to current txn
		err = txn.Set(lockKey, []byte(txn.String()))
	}
	return errors.Trace(err)
}

// RemoveRecord implements table.Table RemoveRecord interface.
func (t *Table) RemoveRecord(ctx context.Context, h int64, r []types.Datum) error {
	err := t.removeRowData(ctx, h)
	if err != nil {
		return errors.Trace(err)
	}

	err = t.removeRowIndices(ctx, h, r)
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (t *Table) removeRowData(ctx context.Context, h int64) error {
	if err := t.LockRow(ctx, h, false); err != nil {
		return errors.Trace(err)
	}
	txn, err := ctx.GetTxn(false)
	if err != nil {
		return errors.Trace(err)
	}
	// Remove row's colume one by one
	for _, col := range t.Columns {
		k := t.RecordKey(h, col)
		err = txn.Delete([]byte(k))
		if err != nil {
			if col.State != model.StatePublic && terror.ErrorEqual(err, kv.ErrNotExist) {
				// If the column is not in public state, we may have not added the column,
				// or already deleted the column, so skip ErrNotExist error.
				continue
			}

			return errors.Trace(err)
		}
	}
	// Remove row lock
	err = txn.Delete([]byte(t.RecordKey(h, nil)))
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// removeRowAllIndex removes all the indices of a row.
func (t *Table) removeRowIndices(ctx context.Context, h int64, rec []types.Datum) error {
	for _, v := range t.indices {
		vals, err := v.FetchValues(rec)
		if vals == nil {
			// TODO: check this
			continue
		}
		txn, err := ctx.GetTxn(false)
		if err != nil {
			return errors.Trace(err)
		}
		if err = v.X.Delete(txn, vals, h); err != nil {
			if v.State != model.StatePublic && terror.ErrorEqual(err, kv.ErrNotExist) {
				// If the index is not in public state, we may have not created the index,
				// or already deleted the index, so skip ErrNotExist error.
				continue
			}

			return errors.Trace(err)
		}
	}
	return nil
}

// RemoveRowIndex implements table.Table RemoveRowIndex interface.
func (t *Table) removeRowIndex(rm kv.RetrieverMutator, h int64, vals []types.Datum, idx *column.IndexedCol) error {
	if err := idx.X.Delete(rm, vals, h); err != nil {
		return errors.Trace(err)
	}
	return nil
}

// BuildIndexForRow implements table.Table BuildIndexForRow interface.
func (t *Table) buildIndexForRow(rm kv.RetrieverMutator, h int64, vals []types.Datum, idx *column.IndexedCol) error {
	if idx.State == model.StateDeleteOnly || idx.State == model.StateDeleteReorganization {
		// If the index is in delete only or write reorganization state, we can not add index.
		return nil
	}

	if err := idx.X.Create(rm, vals, h); err != nil {
		return errors.Trace(err)
	}
	return nil
}

// IterRecords implements table.Table IterRecords interface.
func (t *Table) IterRecords(ctx context.Context, startKey kv.Key, cols []*column.Col,
	fn table.RecordIterFunc) error {
	txn, err := ctx.GetTxn(false)
	if err != nil {
		return errors.Trace(err)
	}
	it, err := txn.Seek(startKey)
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
		handle, err := DecodeRecordKeyHandle(it.Key())
		if err != nil {
			return errors.Trace(err)
		}

		data, err := t.RowWithCols(ctx, handle, cols)
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

// AllocAutoID implements table.Table AllocAutoID interface.
func (t *Table) AllocAutoID() (int64, error) {
	return t.alloc.Alloc(t.ID)
}

// RebaseAutoID implements table.Table RebaseAutoID interface.
func (t *Table) RebaseAutoID(newBase int64, isSetStep bool) error {
	return t.alloc.Rebase(t.ID, newBase, isSetStep)
}

// Seek implements table.Table Seek interface.
func (t *Table) Seek(ctx context.Context, h int64) (int64, bool, error) {
	seekKey := EncodeRecordKey(t.ID, h, 0)
	txn, err := ctx.GetTxn(false)
	if err != nil {
		return 0, false, errors.Trace(err)
	}
	iter, err := txn.Seek(seekKey)
	if !iter.Valid() || !iter.Key().HasPrefix(t.RecordPrefix()) {
		// No more records in the table, skip to the end.
		return 0, false, nil
	}
	handle, err := DecodeRecordKeyHandle(iter.Key())
	if err != nil {
		return 0, false, errors.Trace(err)
	}
	return handle, true, nil
}

var (
	recordPrefixSep = []byte("_r")
	indexPrefixSep  = []byte("_i")
)

// record prefix is "t[tableID]_r"
func genTableRecordPrefix(tableID int64) kv.Key {
	buf := make([]byte, 0, len(TablePrefix)+8+len(recordPrefixSep))
	buf = append(buf, TablePrefix...)
	buf = codec.EncodeInt(buf, tableID)
	buf = append(buf, recordPrefixSep...)
	return buf
}

// index prefix is "t[tableID]_i"
func genTableIndexPrefix(tableID int64) kv.Key {
	buf := make([]byte, 0, len(TablePrefix)+8+len(indexPrefixSep))
	buf = append(buf, TablePrefix...)
	buf = codec.EncodeInt(buf, tableID)
	buf = append(buf, indexPrefixSep...)
	return buf
}

func encodeRecordKey(recordPrefix kv.Key, h int64, columnID int64) kv.Key {
	buf := make([]byte, 0, len(recordPrefix)+16)
	buf = append(buf, recordPrefix...)
	buf = codec.EncodeInt(buf, h)

	if columnID != 0 {
		buf = codec.EncodeInt(buf, columnID)
	}
	return buf
}

// EncodeRecordKey encodes the record key for a table column.
func EncodeRecordKey(tableID int64, h int64, columnID int64) kv.Key {
	prefix := genTableRecordPrefix(tableID)
	return encodeRecordKey(prefix, h, columnID)
}

// DecodeRecordKey decodes the key and gets the tableID, handle and columnID.
func DecodeRecordKey(key kv.Key) (tableID int64, handle int64, columnID int64, err error) {
	k := key
	if !key.HasPrefix(TablePrefix) {
		return 0, 0, 0, errors.Errorf("invalid record key - %q", k)
	}

	key = key[len(TablePrefix):]
	key, tableID, err = codec.DecodeInt(key)
	if err != nil {
		return 0, 0, 0, errors.Trace(err)
	}

	if !key.HasPrefix(recordPrefixSep) {
		return 0, 0, 0, errors.Errorf("invalid record key - %q", k)
	}

	key = key[len(recordPrefixSep):]

	key, handle, err = codec.DecodeInt(key)
	if err != nil {
		return 0, 0, 0, errors.Trace(err)
	}
	if len(key) == 0 {
		return
	}

	key, columnID, err = codec.DecodeInt(key)
	if err != nil {
		return 0, 0, 0, errors.Trace(err)
	}

	return
}

// EncodeValue encodes a go value to bytes.
func EncodeValue(raw types.Datum) ([]byte, error) {
	v, err := flatten(raw)
	if err != nil {
		return nil, errors.Trace(err)
	}
	b, err := codec.EncodeValue(nil, v)
	return b, errors.Trace(err)
}

// DecodeValue implements table.Table DecodeValue interface.
func DecodeValue(data []byte, tp *types.FieldType) (types.Datum, error) {
	values, err := codec.Decode(data)
	if err != nil {
		return types.Datum{}, errors.Trace(err)
	}
	return unflatten(values[0], tp)
}

func flatten(data types.Datum) (types.Datum, error) {
	switch data.Kind() {
	case types.KindMysqlTime:
		// for mysql datetime, timestamp and date type
		b, err := data.GetMysqlTime().Marshal()
		if err != nil {
			return types.NewDatum(nil), errors.Trace(err)
		}
		return types.NewDatum(b), nil
	case types.KindMysqlDuration:
		// for mysql time type
		data.SetInt64(int64(data.GetMysqlDuration().Duration))
		return data, nil
	case types.KindMysqlDecimal:
		data.SetString(data.GetMysqlDecimal().String())
		return data, nil
	case types.KindMysqlEnum:
		data.SetUint64(data.GetMysqlEnum().Value)
		return data, nil
	case types.KindMysqlSet:
		data.SetUint64(data.GetMysqlSet().Value)
		return data, nil
	case types.KindMysqlBit:
		data.SetUint64(data.GetMysqlBit().Value)
		return data, nil
	case types.KindMysqlHex:
		data.SetInt64(data.GetMysqlHex().Value)
		return data, nil
	default:
		return data, nil
	}
}

func unflatten(datum types.Datum, tp *types.FieldType) (types.Datum, error) {
	if datum.Kind() == types.KindNull {
		return datum, nil
	}
	switch tp.Tp {
	case mysql.TypeFloat:
		datum.SetFloat32(float32(datum.GetFloat64()))
		return datum, nil
	case mysql.TypeTiny, mysql.TypeShort, mysql.TypeYear, mysql.TypeInt24, mysql.TypeLong, mysql.TypeLonglong,
		mysql.TypeDouble, mysql.TypeTinyBlob, mysql.TypeMediumBlob, mysql.TypeBlob, mysql.TypeLongBlob,
		mysql.TypeVarchar, mysql.TypeString:
		return datum, nil
	case mysql.TypeDate, mysql.TypeDatetime, mysql.TypeTimestamp:
		var t mysql.Time
		t.Type = tp.Tp
		t.Fsp = tp.Decimal
		err := t.Unmarshal(datum.GetBytes())
		if err != nil {
			return datum, errors.Trace(err)
		}
		datum.SetValue(t)
		return datum, nil
	case mysql.TypeDuration:
		dur := mysql.Duration{Duration: time.Duration(datum.GetInt64())}
		datum.SetValue(dur)
		return datum, nil
	case mysql.TypeNewDecimal, mysql.TypeDecimal:
		dec, err := mysql.ParseDecimal(datum.GetString())
		if err != nil {
			return datum, errors.Trace(err)
		}
		datum.SetValue(dec)
		return datum, nil
	case mysql.TypeEnum:
		enum, err := mysql.ParseEnumValue(tp.Elems, datum.GetUint64())
		if err != nil {
			return datum, errors.Trace(err)
		}
		datum.SetValue(enum)
		return datum, nil
	case mysql.TypeSet:
		set, err := mysql.ParseSetValue(tp.Elems, datum.GetUint64())
		if err != nil {
			return datum, errors.Trace(err)
		}
		datum.SetValue(set)
		return datum, nil
	case mysql.TypeBit:
		bit := mysql.Bit{Value: datum.GetUint64(), Width: tp.Flen}
		datum.SetValue(bit)
		return datum, nil
	}
	log.Error(tp.Tp, datum)
	return datum, nil
}

// SetColValue implements table.Table SetColValue interface.
func SetColValue(rm kv.RetrieverMutator, key []byte, data types.Datum) error {
	v, err := EncodeValue(data)
	if err != nil {
		return errors.Trace(err)
	}
	if err := rm.Set(key, v); err != nil {
		return errors.Trace(err)
	}
	return nil
}

// DecodeRecordKeyHandle decodes the key and gets the record handle.
func DecodeRecordKeyHandle(key kv.Key) (int64, error) {
	_, handle, _, err := DecodeRecordKey(key)
	return handle, errors.Trace(err)
}

// FindIndexByColName implements table.Table FindIndexByColName interface.
func FindIndexByColName(t table.Table, name string) *column.IndexedCol {
	for _, idx := range t.Indices() {
		// only public index can be read.
		if idx.State != model.StatePublic {
			continue
		}

		if len(idx.Columns) == 1 && strings.EqualFold(idx.Columns[0].Name.L, name) {
			return idx
		}
	}
	return nil
}

func init() {
	table.TableFromMeta = TableFromMeta
}
