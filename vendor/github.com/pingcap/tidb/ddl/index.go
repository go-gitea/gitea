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

package ddl

import (
	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/meta"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/table"
	"github.com/pingcap/tidb/table/tables"
	"github.com/pingcap/tidb/terror"
	"github.com/pingcap/tidb/util"
	"github.com/pingcap/tidb/util/types"
)

func buildIndexInfo(tblInfo *model.TableInfo, unique bool, indexName model.CIStr, indexID int64, idxColNames []*ast.IndexColName) (*model.IndexInfo, error) {
	// build offsets
	idxColumns := make([]*model.IndexColumn, 0, len(idxColNames))
	for _, ic := range idxColNames {
		col := findCol(tblInfo.Columns, ic.Column.Name.O)
		if col == nil {
			return nil, errors.Errorf("CREATE INDEX: column does not exist: %s", ic.Column.Name.O)
		}

		idxColumns = append(idxColumns, &model.IndexColumn{
			Name:   col.Name,
			Offset: col.Offset,
			Length: ic.Length,
		})
	}
	// create index info
	idxInfo := &model.IndexInfo{
		ID:      indexID,
		Name:    indexName,
		Columns: idxColumns,
		Unique:  unique,
		State:   model.StateNone,
	}
	return idxInfo, nil
}

func addIndexColumnFlag(tblInfo *model.TableInfo, indexInfo *model.IndexInfo) {
	col := indexInfo.Columns[0]

	if indexInfo.Unique && len(indexInfo.Columns) == 1 {
		tblInfo.Columns[col.Offset].Flag |= mysql.UniqueKeyFlag
	} else {
		tblInfo.Columns[col.Offset].Flag |= mysql.MultipleKeyFlag
	}
}

func dropIndexColumnFlag(tblInfo *model.TableInfo, indexInfo *model.IndexInfo) {
	col := indexInfo.Columns[0]

	if indexInfo.Unique && len(indexInfo.Columns) == 1 {
		tblInfo.Columns[col.Offset].Flag &= ^uint(mysql.UniqueKeyFlag)
	} else {
		tblInfo.Columns[col.Offset].Flag &= ^uint(mysql.MultipleKeyFlag)
	}

	// other index may still cover this col
	for _, index := range tblInfo.Indices {
		if index.Name.L == indexInfo.Name.L {
			continue
		}

		if index.Columns[0].Name.L != col.Name.L {
			continue
		}

		addIndexColumnFlag(tblInfo, index)
	}
}

func (d *ddl) onCreateIndex(t *meta.Meta, job *model.Job) error {
	schemaID := job.SchemaID
	tblInfo, err := d.getTableInfo(t, job)
	if err != nil {
		return errors.Trace(err)
	}

	var (
		unique      bool
		indexName   model.CIStr
		indexID     int64
		idxColNames []*ast.IndexColName
	)

	err = job.DecodeArgs(&unique, &indexName, &indexID, &idxColNames)
	if err != nil {
		job.State = model.JobCancelled
		return errors.Trace(err)
	}

	var indexInfo *model.IndexInfo
	for _, idx := range tblInfo.Indices {
		if idx.Name.L == indexName.L {
			if idx.State == model.StatePublic {
				// we already have a index with same index name
				job.State = model.JobCancelled
				return errors.Errorf("CREATE INDEX: index already exist %s", indexName)
			}

			indexInfo = idx
		}
	}

	if indexInfo == nil {
		indexInfo, err = buildIndexInfo(tblInfo, unique, indexName, indexID, idxColNames)
		if err != nil {
			job.State = model.JobCancelled
			return errors.Trace(err)
		}
		tblInfo.Indices = append(tblInfo.Indices, indexInfo)
	}

	_, err = t.GenSchemaVersion()
	if err != nil {
		return errors.Trace(err)
	}

	switch indexInfo.State {
	case model.StateNone:
		// none -> delete only
		job.SchemaState = model.StateDeleteOnly
		indexInfo.State = model.StateDeleteOnly
		err = t.UpdateTable(schemaID, tblInfo)
		return errors.Trace(err)
	case model.StateDeleteOnly:
		// delete only -> write only
		job.SchemaState = model.StateWriteOnly
		indexInfo.State = model.StateWriteOnly
		err = t.UpdateTable(schemaID, tblInfo)
		return errors.Trace(err)
	case model.StateWriteOnly:
		// write only -> reorganization
		job.SchemaState = model.StateWriteReorganization
		indexInfo.State = model.StateWriteReorganization
		// initialize SnapshotVer to 0 for later reorganization check.
		job.SnapshotVer = 0
		err = t.UpdateTable(schemaID, tblInfo)
		return errors.Trace(err)
	case model.StateWriteReorganization:
		// reorganization -> public
		reorgInfo, err := d.getReorgInfo(t, job)
		if err != nil || reorgInfo.first {
			// if we run reorg firstly, we should update the job snapshot version
			// and then run the reorg next time.
			return errors.Trace(err)
		}

		var tbl table.Table
		tbl, err = d.getTable(schemaID, tblInfo)
		if err != nil {
			return errors.Trace(err)
		}

		err = d.runReorgJob(func() error {
			return d.addTableIndex(tbl, indexInfo, reorgInfo)
		})

		if terror.ErrorEqual(err, errWaitReorgTimeout) {
			// if timeout, we should return, check for the owner and re-wait job done.
			return nil
		}
		if err != nil {
			return errors.Trace(err)
		}

		indexInfo.State = model.StatePublic
		// set column index flag.
		addIndexColumnFlag(tblInfo, indexInfo)
		if err = t.UpdateTable(schemaID, tblInfo); err != nil {
			return errors.Trace(err)
		}

		// finish this job
		job.SchemaState = model.StatePublic
		job.State = model.JobDone
		return nil
	default:
		return errors.Errorf("invalid index state %v", tblInfo.State)
	}
}

func (d *ddl) onDropIndex(t *meta.Meta, job *model.Job) error {
	schemaID := job.SchemaID
	tblInfo, err := d.getTableInfo(t, job)
	if err != nil {
		return errors.Trace(err)
	}

	var indexName model.CIStr
	if err = job.DecodeArgs(&indexName); err != nil {
		job.State = model.JobCancelled
		return errors.Trace(err)
	}

	var indexInfo *model.IndexInfo
	for _, idx := range tblInfo.Indices {
		if idx.Name.L == indexName.L {
			indexInfo = idx
		}
	}

	if indexInfo == nil {
		job.State = model.JobCancelled
		return errors.Errorf("index %s doesn't exist", indexName)
	}

	_, err = t.GenSchemaVersion()
	if err != nil {
		return errors.Trace(err)
	}

	switch indexInfo.State {
	case model.StatePublic:
		// public -> write only
		job.SchemaState = model.StateWriteOnly
		indexInfo.State = model.StateWriteOnly
		err = t.UpdateTable(schemaID, tblInfo)
		return errors.Trace(err)
	case model.StateWriteOnly:
		// write only -> delete only
		job.SchemaState = model.StateDeleteOnly
		indexInfo.State = model.StateDeleteOnly
		err = t.UpdateTable(schemaID, tblInfo)
		return errors.Trace(err)
	case model.StateDeleteOnly:
		// delete only -> reorganization
		job.SchemaState = model.StateDeleteReorganization
		indexInfo.State = model.StateDeleteReorganization
		err = t.UpdateTable(schemaID, tblInfo)
		return errors.Trace(err)
	case model.StateDeleteReorganization:
		// reorganization -> absent
		tbl, err := d.getTable(schemaID, tblInfo)
		if err != nil {
			return errors.Trace(err)
		}

		err = d.runReorgJob(func() error {
			return d.dropTableIndex(tbl, indexInfo)
		})

		if terror.ErrorEqual(err, errWaitReorgTimeout) {
			// if timeout, we should return, check for the owner and re-wait job done.
			return nil
		}
		if err != nil {
			return errors.Trace(err)
		}

		// all reorganization jobs done, drop this index
		newIndices := make([]*model.IndexInfo, 0, len(tblInfo.Indices))
		for _, idx := range tblInfo.Indices {
			if idx.Name.L != indexName.L {
				newIndices = append(newIndices, idx)
			}
		}
		tblInfo.Indices = newIndices
		// set column index flag.
		dropIndexColumnFlag(tblInfo, indexInfo)
		if err = t.UpdateTable(schemaID, tblInfo); err != nil {
			return errors.Trace(err)
		}

		// finish this job
		job.SchemaState = model.StateNone
		job.State = model.JobDone
		return nil
	default:
		return errors.Errorf("invalid table state %v", tblInfo.State)
	}
}

func checkRowExist(txn kv.Transaction, t table.Table, handle int64) (bool, error) {
	_, err := txn.Get(t.RecordKey(handle, nil))
	if terror.ErrorEqual(err, kv.ErrNotExist) {
		// If row doesn't exist, we may have deleted the row already,
		// no need to add index again.
		return false, nil
	} else if err != nil {
		return false, errors.Trace(err)
	}

	return true, nil
}

func fetchRowColVals(txn kv.Transaction, t table.Table, handle int64, indexInfo *model.IndexInfo) ([]types.Datum, error) {
	// fetch datas
	cols := t.Cols()
	vals := make([]types.Datum, 0, len(indexInfo.Columns))
	for _, v := range indexInfo.Columns {
		col := cols[v.Offset]
		k := t.RecordKey(handle, col)
		data, err := txn.Get(k)
		if err != nil {
			return nil, errors.Trace(err)
		}
		val, err := tables.DecodeValue(data, &col.FieldType)
		if err != nil {
			return nil, errors.Trace(err)
		}
		vals = append(vals, val)
	}

	return vals, nil
}

const maxBatchSize = 1024

// How to add index in reorganization state?
//  1. Generate a snapshot with special version.
//  2. Traverse the snapshot, get every row in the table.
//  3. For one row, if the row has been already deleted, skip to next row.
//  4. If not deleted, check whether index has existed, if existed, skip to next row.
//  5. If index doesn't exist, create the index and then continue to handle next row.
func (d *ddl) addTableIndex(t table.Table, indexInfo *model.IndexInfo, reorgInfo *reorgInfo) error {
	seekHandle := reorgInfo.Handle
	version := reorgInfo.SnapshotVer
	for {
		handles, err := d.getSnapshotRows(t, version, seekHandle)
		if err != nil {
			return errors.Trace(err)
		} else if len(handles) == 0 {
			return nil
		}

		seekHandle = handles[len(handles)-1] + 1

		err = d.backfillTableIndex(t, indexInfo, handles, reorgInfo)
		if err != nil {
			return errors.Trace(err)
		}
	}
}

func (d *ddl) getSnapshotRows(t table.Table, version uint64, seekHandle int64) ([]int64, error) {
	ver := kv.Version{Ver: version}

	snap, err := d.store.GetSnapshot(ver)
	if err != nil {
		return nil, errors.Trace(err)
	}

	defer snap.Release()

	firstKey := t.RecordKey(seekHandle, nil)

	it, err := snap.Seek(firstKey)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer it.Close()

	handles := make([]int64, 0, maxBatchSize)

	for it.Valid() {
		if !it.Key().HasPrefix(t.RecordPrefix()) {
			break
		}

		var handle int64
		handle, err = tables.DecodeRecordKeyHandle(it.Key())
		if err != nil {
			return nil, errors.Trace(err)
		}

		rk := t.RecordKey(handle, nil)

		handles = append(handles, handle)
		if len(handles) == maxBatchSize {
			break
		}

		err = kv.NextUntil(it, util.RowKeyPrefixFilter(rk))
		if terror.ErrorEqual(err, kv.ErrNotExist) {
			break
		} else if err != nil {
			return nil, errors.Trace(err)
		}
	}

	return handles, nil
}

func lockRow(txn kv.Transaction, t table.Table, h int64) error {
	// Get row lock key
	lockKey := t.RecordKey(h, nil)
	// set row lock key to current txn
	err := txn.Set(lockKey, []byte(txn.String()))
	return errors.Trace(err)
}

func (d *ddl) backfillTableIndex(t table.Table, indexInfo *model.IndexInfo, handles []int64, reorgInfo *reorgInfo) error {
	kvX := kv.NewKVIndex(t.IndexPrefix(), indexInfo.Name.L, indexInfo.ID, indexInfo.Unique)

	for _, handle := range handles {
		log.Debug("[ddl] building index...", handle)

		err := kv.RunInNewTxn(d.store, true, func(txn kv.Transaction) error {
			if err := d.isReorgRunnable(txn); err != nil {
				return errors.Trace(err)
			}

			// first check row exists
			exist, err := checkRowExist(txn, t, handle)
			if err != nil {
				return errors.Trace(err)
			} else if !exist {
				// row doesn't exist, skip it.
				return nil
			}

			var vals []types.Datum
			vals, err = fetchRowColVals(txn, t, handle, indexInfo)
			if err != nil {
				return errors.Trace(err)
			}

			exist, _, err = kvX.Exist(txn, vals, handle)
			if err != nil {
				return errors.Trace(err)
			} else if exist {
				// index already exists, skip it.
				return nil
			}

			err = lockRow(txn, t, handle)
			if err != nil {
				return errors.Trace(err)
			}

			// create the index.
			err = kvX.Create(txn, vals, handle)
			if err != nil {
				return errors.Trace(err)
			}

			// update reorg next handle
			return errors.Trace(reorgInfo.UpdateHandle(txn, handle))
		})

		if err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}

func (d *ddl) dropTableIndex(t table.Table, indexInfo *model.IndexInfo) error {
	prefix := kv.GenIndexPrefix(t.IndexPrefix(), indexInfo.ID)
	err := d.delKeysWithPrefix(prefix)

	return errors.Trace(err)
}
