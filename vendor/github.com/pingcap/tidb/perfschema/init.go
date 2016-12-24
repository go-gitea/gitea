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

package perfschema

import (
	"github.com/juju/errors"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/meta"
	"github.com/pingcap/tidb/meta/autoid"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/table"
	"github.com/pingcap/tidb/table/tables"
	"github.com/pingcap/tidb/util/charset"
	"github.com/pingcap/tidb/util/types"
)

type columnInfo struct {
	tp    byte
	size  int
	flag  uint
	deflt interface{}
	elems []string
}

var setupActorsCols = []columnInfo{
	{mysql.TypeString, 60, mysql.NotNullFlag, `%`, nil},
	{mysql.TypeString, 32, mysql.NotNullFlag, `%`, nil},
	{mysql.TypeString, 16, mysql.NotNullFlag, `%`, nil},
	{mysql.TypeEnum, -1, mysql.NotNullFlag, "YES", []string{"YES", "NO"}},
	{mysql.TypeEnum, -1, mysql.NotNullFlag, "YES", []string{"YES", "NO"}},
}

var setupObjectsCols = []columnInfo{
	{mysql.TypeEnum, -1, mysql.NotNullFlag, "TABLE", []string{"EVENT", "FUNCTION", "TABLE"}},
	{mysql.TypeVarchar, 64, 0, `%`, nil},
	{mysql.TypeVarchar, 64, mysql.NotNullFlag, `%`, nil},
	{mysql.TypeEnum, -1, mysql.NotNullFlag, "YES", []string{"YES", "NO"}},
	{mysql.TypeEnum, -1, mysql.NotNullFlag, "YES", []string{"YES", "NO"}},
}

var setupInstrumentsCols = []columnInfo{
	{mysql.TypeVarchar, 128, mysql.NotNullFlag, nil, nil},
	{mysql.TypeEnum, -1, mysql.NotNullFlag, nil, []string{"YES", "NO"}},
	{mysql.TypeEnum, -1, mysql.NotNullFlag, nil, []string{"YES", "NO"}},
}

var setupConsumersCols = []columnInfo{
	{mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{mysql.TypeEnum, -1, mysql.NotNullFlag, nil, []string{"YES", "NO"}},
}

var setupTimersCols = []columnInfo{
	{mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{mysql.TypeEnum, -1, mysql.NotNullFlag, nil, []string{"NANOSECOND", "MICROSECOND", "MILLISECOND"}},
}

var stmtsCurrentCols = []columnInfo{
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.UnsignedFlag, nil, nil},
	{mysql.TypeVarchar, 128, mysql.NotNullFlag, nil, nil},
	{mysql.TypeVarchar, 64, 0, nil, nil},
	{mysql.TypeLonglong, 20, mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLongBlob, -1, 0, nil, nil},
	{mysql.TypeVarchar, 32, 0, nil, nil},
	{mysql.TypeLongBlob, -1, 0, nil, nil},
	{mysql.TypeVarchar, 64, 0, nil, nil},
	{mysql.TypeVarchar, 64, 0, nil, nil},
	{mysql.TypeVarchar, 64, 0, nil, nil},
	{mysql.TypeVarchar, 64, 0, nil, nil},
	{mysql.TypeLonglong, 20, mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLong, 11, 0, nil, nil},
	{mysql.TypeVarchar, 5, 0, nil, nil},
	{mysql.TypeVarchar, 128, 0, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.UnsignedFlag, nil, nil},
	{mysql.TypeEnum, -1, 0, nil, []string{"TRANSACTION", "STATEMENT", "STAGE"}},
	{mysql.TypeLong, 11, 0, nil, nil},
}

var preparedStmtsInstancesCols = []columnInfo{
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeVarchar, 64, 0, nil, nil},
	{mysql.TypeLongBlob, -1, mysql.NotNullFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeEnum, -1, 0, nil, []string{"EVENT", "FUNCTION", "TABLE"}},
	{mysql.TypeVarchar, 64, 0, nil, nil},
	{mysql.TypeVarchar, 64, 0, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
}

var transCurrentCols = []columnInfo{
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.UnsignedFlag, nil, nil},
	{mysql.TypeVarchar, 128, mysql.NotNullFlag, nil, nil},
	{mysql.TypeEnum, -1, 0, nil, []string{"ACTIVE", "COMMITTED", "ROLLED BACK"}},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeVarchar, 64, 0, nil, nil},
	{mysql.TypeLong, 11, 0, nil, nil},
	{mysql.TypeVarchar, 130, 0, nil, nil},
	{mysql.TypeVarchar, 130, 0, nil, nil},
	{mysql.TypeVarchar, 64, 0, nil, nil},
	{mysql.TypeVarchar, 64, 0, nil, nil},
	{mysql.TypeLonglong, 20, mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.UnsignedFlag, nil, nil},
	{mysql.TypeEnum, -1, 0, nil, []string{"READ ONLY", "READ WRITE"}},
	{mysql.TypeVarchar, 64, 0, nil, nil},
	{mysql.TypeEnum, -1, mysql.NotNullFlag, nil, []string{"YES", "NO"}},
	{mysql.TypeLonglong, 20, mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.UnsignedFlag, nil, nil},
	{mysql.TypeEnum, -1, 0, nil, []string{"TRANSACTION", "STATEMENT", "STAGE"}},
}

var stagesCurrentCols = []columnInfo{
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.NotNullFlag | mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.UnsignedFlag, nil, nil},
	{mysql.TypeVarchar, 128, mysql.NotNullFlag, nil, nil},
	{mysql.TypeVarchar, 64, 0, nil, nil},
	{mysql.TypeLonglong, 20, mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.UnsignedFlag, nil, nil},
	{mysql.TypeLonglong, 20, mysql.UnsignedFlag, nil, nil},
	{mysql.TypeEnum, -1, 0, nil, []string{"TRANSACTION", "STATEMENT", "STAGE"}},
}

func setColumnID(meta *model.TableInfo, store kv.Storage) error {
	var err error
	for _, c := range meta.Columns {
		c.ID, err = genGlobalID(store)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func genGlobalID(store kv.Storage) (int64, error) {
	var globalID int64
	err := kv.RunInNewTxn(store, true, func(txn kv.Transaction) error {
		var err error
		globalID, err = meta.NewMeta(txn).GenGlobalID()
		return errors.Trace(err)
	})
	return globalID, errors.Trace(err)
}

func createMemoryTable(meta *model.TableInfo, alloc autoid.Allocator) (table.Table, error) {
	tbl, _ := tables.MemoryTableFromMeta(alloc, meta)
	return tbl, nil
}

func (ps *perfSchema) buildTables() error {
	tbls := make([]*model.TableInfo, 0, len(ps.tables))
	ps.mTables = make(map[string]table.Table, len(ps.tables))
	dbID, err := genGlobalID(ps.store)
	if err != nil {
		return errors.Trace(err)
	}
	// Set PKIsHandle
	// TableStmtsCurrent use THREAD_ID as PK and handle
	tb := ps.tables[TableStmtsHistory]
	tb.PKIsHandle = true
	tb.Columns[0].Flag = tb.Columns[0].Flag | mysql.PriKeyFlag

	var tbl table.Table
	for name, meta := range ps.tables {
		tbls = append(tbls, meta)
		meta.ID, err = genGlobalID(ps.store)
		if err != nil {
			return errors.Trace(err)
		}
		err = setColumnID(meta, ps.store)
		if err != nil {
			return errors.Trace(err)
		}
		alloc := autoid.NewMemoryAllocator(dbID)
		tbl, err = createMemoryTable(meta, alloc)
		if err != nil {
			return errors.Trace(err)
		}
		ps.mTables[name] = tbl
	}
	ps.dbInfo = &model.DBInfo{
		ID:      dbID,
		Name:    model.NewCIStr(Name),
		Charset: mysql.DefaultCharset,
		Collate: mysql.DefaultCollationName,
		Tables:  tbls,
	}
	return nil
}

func (ps *perfSchema) buildModel(tbName string, colNames []string, cols []columnInfo) {
	rcols := make([]*model.ColumnInfo, len(cols))
	for i, col := range cols {
		var ci *model.ColumnInfo
		if col.elems == nil {
			ci = buildUsualColumnInfo(i, colNames[i], col.tp, col.size, col.flag, col.deflt)
		} else {
			ci = buildEnumColumnInfo(i, colNames[i], col.elems, col.flag, col.deflt)
		}
		rcols[i] = ci
	}

	ps.tables[tbName] = &model.TableInfo{
		Name:    model.NewCIStr(tbName),
		Charset: "utf8",
		Collate: "utf8",
		Columns: rcols,
	}
}

func buildUsualColumnInfo(offset int, name string, tp byte, size int, flag uint, def interface{}) *model.ColumnInfo {
	mCharset := charset.CharsetBin
	mCollation := charset.CharsetBin
	if tp == mysql.TypeString || tp == mysql.TypeVarchar || tp == mysql.TypeBlob || tp == mysql.TypeLongBlob {
		mCharset = mysql.DefaultCharset
		mCollation = mysql.DefaultCollationName
	}
	if def == nil {
		flag |= mysql.NoDefaultValueFlag
	}
	// TODO: does TypeLongBlob need size?
	fieldType := types.FieldType{
		Charset: mCharset,
		Collate: mCollation,
		Tp:      tp,
		Flen:    size,
		Flag:    uint(flag),
	}
	colInfo := &model.ColumnInfo{
		Name:         model.NewCIStr(name),
		Offset:       offset,
		FieldType:    fieldType,
		DefaultValue: def,
		State:        model.StatePublic,
	}
	return colInfo
}

func buildEnumColumnInfo(offset int, name string, elems []string, flag uint, def interface{}) *model.ColumnInfo {
	mCharset := charset.CharsetBin
	mCollation := charset.CharsetBin
	if def == nil {
		flag |= mysql.NoDefaultValueFlag
	}
	fieldType := types.FieldType{
		Charset: mCharset,
		Collate: mCollation,
		Tp:      mysql.TypeEnum,
		Flag:    uint(flag),
		Elems:   elems,
	}
	colInfo := &model.ColumnInfo{
		Name:         model.NewCIStr(name),
		Offset:       offset,
		FieldType:    fieldType,
		DefaultValue: def,
		State:        model.StatePublic,
	}
	return colInfo
}

func (ps *perfSchema) initRecords(tbName string, records [][]types.Datum) error {
	tbl, ok := ps.mTables[tbName]
	if !ok {
		return errors.Errorf("Unknown PerformanceSchema table: %s", tbName)
	}
	for _, rec := range records {
		_, err := tbl.AddRecord(nil, rec)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

var setupTimersRecords [][]types.Datum

func (ps *perfSchema) initialize() (err error) {
	ps.tables = make(map[string]*model.TableInfo)

	allColDefs := [][]columnInfo{
		setupActorsCols,
		setupObjectsCols,
		setupInstrumentsCols,
		setupConsumersCols,
		setupTimersCols,
		stmtsCurrentCols,
		stmtsCurrentCols, // same as above
		stmtsCurrentCols, // same as above
		preparedStmtsInstancesCols,
		transCurrentCols,
		transCurrentCols, // same as above
		transCurrentCols, // same as above
		stagesCurrentCols,
		stagesCurrentCols, // same as above
		stagesCurrentCols, // same as above
	}

	allColNames := [][]string{
		ColumnSetupActors,
		ColumnSetupObjects,
		ColumnSetupInstruments,
		ColumnSetupConsumers,
		ColumnSetupTimers,
		ColumnStmtsCurrent,
		ColumnStmtsHistory,
		ColumnStmtsHistoryLong,
		ColumnPreparedStmtsInstances,
		ColumnStmtsCurrent,
		ColumnStmtsHistory,
		ColumnStmtsHistoryLong,
		ColumnStagesCurrent,
		ColumnStagesHistory,
		ColumnStagesHistoryLong,
	}

	// initialize all table, column and result field definitions
	for i, def := range allColDefs {
		ps.buildModel(PerfSchemaTables[i], allColNames[i], def)
	}
	err = ps.buildTables()
	if err != nil {
		return errors.Trace(err)
	}

	setupActorsRecords := [][]types.Datum{
		types.MakeDatums(`%`, `%`, `%`, mysql.Enum{Name: "YES", Value: 1}, mysql.Enum{Name: "YES", Value: 1}),
	}
	err = ps.initRecords(TableSetupActors, setupActorsRecords)
	if err != nil {
		return errors.Trace(err)
	}

	setupObjectsRecords := [][]types.Datum{
		types.MakeDatums(mysql.Enum{Name: "EVENT", Value: 1}, "mysql", `%`, mysql.Enum{Name: "NO", Value: 2}, mysql.Enum{Name: "NO", Value: 2}),
		types.MakeDatums(mysql.Enum{Name: "EVENT", Value: 1}, "performance_schema", `%`, mysql.Enum{Name: "NO", Value: 2}, mysql.Enum{Name: "NO", Value: 2}),
		types.MakeDatums(mysql.Enum{Name: "EVENT", Value: 1}, "information_schema", `%`, mysql.Enum{Name: "NO", Value: 2}, mysql.Enum{Name: "NO", Value: 2}),
		types.MakeDatums(mysql.Enum{Name: "EVENT", Value: 1}, `%`, `%`, mysql.Enum{Name: "YES", Value: 1}, mysql.Enum{Name: "YES", Value: 1}),
		types.MakeDatums(mysql.Enum{Name: "FUNCTION", Value: 2}, "mysql", `%`, mysql.Enum{Name: "NO", Value: 2}, mysql.Enum{Name: "NO", Value: 2}),
		types.MakeDatums(mysql.Enum{Name: "FUNCTION", Value: 2}, "performance_schema", `%`, mysql.Enum{Name: "NO", Value: 2}, mysql.Enum{Name: "NO", Value: 2}),
		types.MakeDatums(mysql.Enum{Name: "FUNCTION", Value: 2}, "information_schema", `%`, mysql.Enum{Name: "NO", Value: 2}, mysql.Enum{Name: "NO", Value: 2}),
		types.MakeDatums(mysql.Enum{Name: "FUNCTION", Value: 2}, `%`, `%`, mysql.Enum{Name: "YES", Value: 1}, mysql.Enum{Name: "YES", Value: 1}),
		types.MakeDatums(mysql.Enum{Name: "TABLE", Value: 3}, "mysql", `%`, mysql.Enum{Name: "NO", Value: 2}, mysql.Enum{Name: "NO", Value: 2}),
		types.MakeDatums(mysql.Enum{Name: "TABLE", Value: 3}, "performance_schema", `%`, mysql.Enum{Name: "NO", Value: 2}, mysql.Enum{Name: "NO", Value: 2}),
		types.MakeDatums(mysql.Enum{Name: "TABLE", Value: 3}, "information_schema", `%`, mysql.Enum{Name: "NO", Value: 2}, mysql.Enum{Name: "NO", Value: 2}),
		types.MakeDatums(mysql.Enum{Name: "TABLE", Value: 3}, `%`, `%`, mysql.Enum{Name: "YES", Value: 1}, mysql.Enum{Name: "YES", Value: 1}),
	}
	err = ps.initRecords(TableSetupObjects, setupObjectsRecords)
	if err != nil {
		return errors.Trace(err)
	}

	setupConsumersRecords := [][]types.Datum{
		types.MakeDatums("events_stages_current", mysql.Enum{Name: "NO", Value: 2}),
		types.MakeDatums("events_stages_history", mysql.Enum{Name: "NO", Value: 2}),
		types.MakeDatums("events_stages_history_long", mysql.Enum{Name: "NO", Value: 2}),
		types.MakeDatums("events_statements_current", mysql.Enum{Name: "YES", Value: 1}),
		types.MakeDatums("events_statements_history", mysql.Enum{Name: "YES", Value: 1}),
		types.MakeDatums("events_statements_history_long", mysql.Enum{Name: "NO", Value: 2}),
		types.MakeDatums("events_transactions_current", mysql.Enum{Name: "YES", Value: 1}),
		types.MakeDatums("events_transactions_history", mysql.Enum{Name: "YES", Value: 1}),
		types.MakeDatums("events_transactions_history_long", mysql.Enum{Name: "YES", Value: 1}),
		types.MakeDatums("global_instrumentation", mysql.Enum{Name: "YES", Value: 1}),
		types.MakeDatums("thread_instrumentation", mysql.Enum{Name: "YES", Value: 1}),
		types.MakeDatums("statements_digest", mysql.Enum{Name: "YES", Value: 1}),
	}
	err = ps.initRecords(TableSetupConsumers, setupConsumersRecords)
	if err != nil {
		return errors.Trace(err)
	}

	setupTimersRecords = [][]types.Datum{
		types.MakeDatums("stage", mysql.Enum{Name: "NANOSECOND", Value: 1}),
		types.MakeDatums("statement", mysql.Enum{Name: "NANOSECOND", Value: 1}),
		types.MakeDatums("transaction", mysql.Enum{Name: "NANOSECOND", Value: 1}),
	}
	err = ps.initRecords(TableSetupTimers, setupTimersRecords)
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (ps *perfSchema) GetDBMeta() *model.DBInfo {
	return ps.dbInfo
}

func (ps *perfSchema) GetTable(name string) (table.Table, bool) {
	tbl, ok := ps.mTables[name]
	return tbl, ok
}
