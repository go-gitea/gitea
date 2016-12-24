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

package infoschema

import (
	"strings"
	"sync/atomic"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/meta"
	"github.com/pingcap/tidb/meta/autoid"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/perfschema"
	"github.com/pingcap/tidb/table"
	"github.com/pingcap/tidb/terror"
	// import table implementation to init table.TableFromMeta
	_ "github.com/pingcap/tidb/table/tables"
	"github.com/pingcap/tidb/util/types"
)

// InfoSchema is the interface used to retrieve the schema information.
// It works as a in memory cache and doesn't handle any schema change.
// InfoSchema is read-only, and the returned value is a copy.
// TODO: add more methods to retrieve tables and columns.
type InfoSchema interface {
	SchemaByName(schema model.CIStr) (*model.DBInfo, bool)
	SchemaExists(schema model.CIStr) bool
	TableByName(schema, table model.CIStr) (table.Table, error)
	TableExists(schema, table model.CIStr) bool
	ColumnByName(schema, table, column model.CIStr) (*model.ColumnInfo, bool)
	ColumnExists(schema, table, column model.CIStr) bool
	IndexByName(schema, table, index model.CIStr) (*model.IndexInfo, bool)
	SchemaByID(id int64) (*model.DBInfo, bool)
	TableByID(id int64) (table.Table, bool)
	AllocByID(id int64) (autoid.Allocator, bool)
	ColumnByID(id int64) (*model.ColumnInfo, bool)
	ColumnIndicesByID(id int64) ([]*model.IndexInfo, bool)
	AllSchemaNames() []string
	AllSchemas() []*model.DBInfo
	Clone() (result []*model.DBInfo)
	SchemaTables(schema model.CIStr) []table.Table
	SchemaMetaVersion() int64
}

// Infomation Schema Name.
const (
	Name = "INFORMATION_SCHEMA"
)

type infoSchema struct {
	schemaNameToID  map[string]int64
	tableNameToID   map[tableName]int64
	columnNameToID  map[columnName]int64
	schemas         map[int64]*model.DBInfo
	tables          map[int64]table.Table
	tableAllocators map[int64]autoid.Allocator
	columns         map[int64]*model.ColumnInfo
	indices         map[indexName]*model.IndexInfo
	columnIndices   map[int64][]*model.IndexInfo

	// We should check version when change schema.
	schemaMetaVersion int64
}

var _ InfoSchema = (*infoSchema)(nil)

type tableName struct {
	schema string
	table  string
}

type columnName struct {
	tableName
	name string
}

type indexName struct {
	tableName
	name string
}

func (is *infoSchema) SchemaByName(schema model.CIStr) (val *model.DBInfo, ok bool) {
	id, ok := is.schemaNameToID[schema.L]
	if !ok {
		return
	}
	val, ok = is.schemas[id]
	return
}

func (is *infoSchema) SchemaMetaVersion() int64 {
	return is.schemaMetaVersion
}

func (is *infoSchema) SchemaExists(schema model.CIStr) bool {
	_, ok := is.schemaNameToID[schema.L]
	return ok
}

func (is *infoSchema) TableByName(schema, table model.CIStr) (t table.Table, err error) {
	id, ok := is.tableNameToID[tableName{schema: schema.L, table: table.L}]
	if !ok {
		return nil, TableNotExists.Gen("table %s.%s does not exist", schema, table)
	}
	t = is.tables[id]
	return
}

func (is *infoSchema) TableExists(schema, table model.CIStr) bool {
	_, ok := is.tableNameToID[tableName{schema: schema.L, table: table.L}]
	return ok
}

func (is *infoSchema) ColumnByName(schema, table, column model.CIStr) (val *model.ColumnInfo, ok bool) {
	id, ok := is.columnNameToID[columnName{tableName: tableName{schema: schema.L, table: table.L}, name: column.L}]
	if !ok {
		return
	}
	val, ok = is.columns[id]
	return
}

func (is *infoSchema) ColumnExists(schema, table, column model.CIStr) bool {
	_, ok := is.columnNameToID[columnName{tableName: tableName{schema: schema.L, table: table.L}, name: column.L}]
	return ok
}

func (is *infoSchema) IndexByName(schema, table, index model.CIStr) (val *model.IndexInfo, ok bool) {
	val, ok = is.indices[indexName{tableName: tableName{schema: schema.L, table: table.L}, name: index.L}]
	return
}

func (is *infoSchema) SchemaByID(id int64) (val *model.DBInfo, ok bool) {
	val, ok = is.schemas[id]
	return
}

func (is *infoSchema) TableByID(id int64) (val table.Table, ok bool) {
	val, ok = is.tables[id]
	return
}

func (is *infoSchema) AllocByID(id int64) (val autoid.Allocator, ok bool) {
	val, ok = is.tableAllocators[id]
	return
}

func (is *infoSchema) ColumnByID(id int64) (val *model.ColumnInfo, ok bool) {
	val, ok = is.columns[id]
	return
}

func (is *infoSchema) ColumnIndicesByID(id int64) (indices []*model.IndexInfo, ok bool) {
	indices, ok = is.columnIndices[id]
	return
}

func (is *infoSchema) AllSchemaNames() (names []string) {
	for _, v := range is.schemas {
		names = append(names, v.Name.O)
	}
	return
}

func (is *infoSchema) AllSchemas() (schemas []*model.DBInfo) {
	for _, v := range is.schemas {
		schemas = append(schemas, v)
	}
	return
}

func (is *infoSchema) SchemaTables(schema model.CIStr) (tables []table.Table) {
	di, ok := is.SchemaByName(schema)
	if !ok {
		return
	}
	for _, ti := range di.Tables {
		tables = append(tables, is.tables[ti.ID])
	}
	return
}

func (is *infoSchema) Clone() (result []*model.DBInfo) {
	for _, v := range is.schemas {
		result = append(result, v.Clone())
	}
	return
}

// Handle handles information schema, including getting and setting.
type Handle struct {
	value atomic.Value
	store kv.Storage
}

// NewHandle creates a new Handle.
func NewHandle(store kv.Storage) *Handle {
	h := &Handle{
		store: store,
	}
	// init memory tables
	initMemoryTables(store)
	initPerfSchema(store)
	return h
}

func initPerfSchema(store kv.Storage) {
	perfHandle = perfschema.NewPerfHandle(store)
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

var (
	// Information_Schema
	isDB          *model.DBInfo
	schemataTbl   table.Table
	tablesTbl     table.Table
	columnsTbl    table.Table
	statisticsTbl table.Table
	charsetTbl    table.Table
	collationsTbl table.Table
	filesTbl      table.Table
	defTbl        table.Table
	profilingTbl  table.Table
	nameToTable   map[string]table.Table

	perfHandle perfschema.PerfSchema
)

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

func initMemoryTables(store kv.Storage) error {
	// Init Information_Schema
	var (
		err error
		tbl table.Table
	)
	dbID, err := genGlobalID(store)
	if err != nil {
		return errors.Trace(err)
	}
	nameToTable = make(map[string]table.Table)
	isTables := make([]*model.TableInfo, 0, len(tableNameToColumns))
	for name, cols := range tableNameToColumns {
		meta := buildTableMeta(name, cols)
		isTables = append(isTables, meta)
		meta.ID, err = genGlobalID(store)
		if err != nil {
			return errors.Trace(err)
		}
		err = setColumnID(meta, store)
		if err != nil {
			return errors.Trace(err)
		}
		alloc := autoid.NewMemoryAllocator(dbID)
		tbl, err = createMemoryTable(meta, alloc)
		if err != nil {
			return errors.Trace(err)
		}
		nameToTable[meta.Name.L] = tbl
	}
	schemataTbl = nameToTable[strings.ToLower(tableSchemata)]
	tablesTbl = nameToTable[strings.ToLower(tableTables)]
	columnsTbl = nameToTable[strings.ToLower(tableColumns)]
	statisticsTbl = nameToTable[strings.ToLower(tableStatistics)]
	charsetTbl = nameToTable[strings.ToLower(tableCharacterSets)]
	collationsTbl = nameToTable[strings.ToLower(tableCollations)]

	// CharacterSets/Collations contain static data. Init them now.
	err = insertData(charsetTbl, dataForCharacterSets())
	if err != nil {
		return errors.Trace(err)
	}
	err = insertData(collationsTbl, dataForColltions())
	if err != nil {
		return errors.Trace(err)
	}
	// create db
	isDB = &model.DBInfo{
		ID:      dbID,
		Name:    model.NewCIStr(Name),
		Charset: mysql.DefaultCharset,
		Collate: mysql.DefaultCollationName,
		Tables:  isTables,
	}
	return nil
}

func insertData(tbl table.Table, rows [][]types.Datum) error {
	for _, r := range rows {
		_, err := tbl.AddRecord(nil, r)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func refillTable(tbl table.Table, rows [][]types.Datum) error {
	err := tbl.Truncate(nil)
	if err != nil {
		return errors.Trace(err)
	}
	return insertData(tbl, rows)
}

// Set sets DBInfo to information schema.
func (h *Handle) Set(newInfo []*model.DBInfo, schemaMetaVersion int64) error {
	info := &infoSchema{
		schemaNameToID:    map[string]int64{},
		tableNameToID:     map[tableName]int64{},
		columnNameToID:    map[columnName]int64{},
		schemas:           map[int64]*model.DBInfo{},
		tables:            map[int64]table.Table{},
		tableAllocators:   map[int64]autoid.Allocator{},
		columns:           map[int64]*model.ColumnInfo{},
		indices:           map[indexName]*model.IndexInfo{},
		columnIndices:     map[int64][]*model.IndexInfo{},
		schemaMetaVersion: schemaMetaVersion,
	}
	var err error
	var hasOldInfo bool
	infoschema := h.Get()
	if infoschema != nil {
		hasOldInfo = true
	}
	for _, di := range newInfo {
		info.schemas[di.ID] = di
		info.schemaNameToID[di.Name.L] = di.ID
		for _, t := range di.Tables {
			alloc := autoid.NewAllocator(h.store, di.ID)
			if hasOldInfo {
				val, ok := infoschema.AllocByID(t.ID)
				if ok {
					alloc = val
				}
			}
			info.tableAllocators[t.ID] = alloc
			info.tables[t.ID], err = table.TableFromMeta(alloc, t)
			if err != nil {
				return errors.Trace(err)
			}
			tname := tableName{di.Name.L, t.Name.L}
			info.tableNameToID[tname] = t.ID
			for _, c := range t.Columns {
				info.columns[c.ID] = c
				info.columnNameToID[columnName{tname, c.Name.L}] = c.ID
			}
			for _, idx := range t.Indices {
				info.indices[indexName{tname, idx.Name.L}] = idx
				for _, idxCol := range idx.Columns {
					columnID := t.Columns[idxCol.Offset].ID
					columnIndices := info.columnIndices[columnID]
					info.columnIndices[columnID] = append(columnIndices, idx)
				}
			}
		}
	}
	// Build Information_Schema
	info.schemaNameToID[isDB.Name.L] = isDB.ID
	info.schemas[isDB.ID] = isDB
	for _, t := range isDB.Tables {
		tbl, ok := nameToTable[t.Name.L]
		if !ok {
			return errors.Errorf("table `%s` is missing.", t.Name)
		}
		info.tables[t.ID] = tbl
		tname := tableName{isDB.Name.L, t.Name.L}
		info.tableNameToID[tname] = t.ID
		for _, c := range t.Columns {
			info.columns[c.ID] = c
			info.columnNameToID[columnName{tname, c.Name.L}] = c.ID
		}
	}

	// Add Performance_Schema
	psDB := perfHandle.GetDBMeta()
	info.schemaNameToID[psDB.Name.L] = psDB.ID
	info.schemas[psDB.ID] = psDB
	for _, t := range psDB.Tables {
		tbl, ok := perfHandle.GetTable(t.Name.O)
		if !ok {
			return errors.Errorf("table `%s` is missing.", t.Name)
		}
		info.tables[t.ID] = tbl
		tname := tableName{psDB.Name.L, t.Name.L}
		info.tableNameToID[tname] = t.ID
		for _, c := range t.Columns {
			info.columns[c.ID] = c
			info.columnNameToID[columnName{tname, c.Name.L}] = c.ID
		}
	}
	// Should refill some tables in Information_Schema.
	// schemata/tables/columns/statistics
	dbNames := make([]string, 0, len(info.schemas))
	dbInfos := make([]*model.DBInfo, 0, len(info.schemas))
	for _, v := range info.schemas {
		dbNames = append(dbNames, v.Name.L)
		dbInfos = append(dbInfos, v)
	}
	err = refillTable(schemataTbl, dataForSchemata(dbNames))
	if err != nil {
		return errors.Trace(err)
	}
	err = refillTable(tablesTbl, dataForTables(dbInfos))
	if err != nil {
		return errors.Trace(err)
	}
	err = refillTable(columnsTbl, dataForColumns(dbInfos))
	if err != nil {
		return errors.Trace(err)
	}
	err = refillTable(statisticsTbl, dataForStatistics(dbInfos))
	if err != nil {
		return errors.Trace(err)
	}
	h.value.Store(info)
	return nil
}

// Get gets information schema from Handle.
func (h *Handle) Get() InfoSchema {
	v := h.value.Load()
	schema, _ := v.(InfoSchema)
	return schema
}

// Schema error codes.
const (
	CodeDbDropExists      terror.ErrCode = 1008
	CodeDatabaseNotExists                = 1049
	CodeTableNotExists                   = 1146
	CodeColumnNotExists                  = 1054

	CodeDatabaseExists = 1007
	CodeTableExists    = 1050
	CodeBadTable       = 1051
)

var (
	// DatabaseDropExists returns for drop an unexist database.
	DatabaseDropExists = terror.ClassSchema.New(CodeDbDropExists, "database doesn't exist")
	// DatabaseNotExists returns for database not exists.
	DatabaseNotExists = terror.ClassSchema.New(CodeDatabaseNotExists, "database not exists")
	// TableNotExists returns for table not exists.
	TableNotExists = terror.ClassSchema.New(CodeTableNotExists, "table not exists")
	// ColumnNotExists returns for column not exists.
	ColumnNotExists = terror.ClassSchema.New(CodeColumnNotExists, "field not exists")

	// DatabaseExists returns for database already exists.
	DatabaseExists = terror.ClassSchema.New(CodeDatabaseExists, "database already exists")
	// TableExists returns for table already exists.
	TableExists = terror.ClassSchema.New(CodeTableExists, "table already exists")
	// TableDropExists returns for drop an unexist table.
	TableDropExists = terror.ClassSchema.New(CodeBadTable, "unknown table")
)

func init() {
	schemaMySQLErrCodes := map[terror.ErrCode]uint16{
		CodeDbDropExists:      mysql.ErrDbDropExists,
		CodeDatabaseNotExists: mysql.ErrBadDb,
		CodeTableNotExists:    mysql.ErrNoSuchTable,
		CodeColumnNotExists:   mysql.ErrBadField,
		CodeDatabaseExists:    mysql.ErrDbCreateExists,
		CodeTableExists:       mysql.ErrTableExists,
		CodeBadTable:          mysql.ErrBadTable,
	}
	terror.ErrClassToMySQLCodes[terror.ClassSchema] = schemaMySQLErrCodes
}
