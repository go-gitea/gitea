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

package infoschema

import (
	"fmt"
	"sort"

	"github.com/pingcap/tidb/column"
	"github.com/pingcap/tidb/meta/autoid"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/table"
	"github.com/pingcap/tidb/table/tables"
	"github.com/pingcap/tidb/util/charset"
	"github.com/pingcap/tidb/util/types"
)

const (
	tableSchemata      = "SCHEMATA"
	tableTables        = "TABLES"
	tableColumns       = "COLUMNS"
	tableStatistics    = "STATISTICS"
	tableCharacterSets = "CHARACTER_SETS"
	tableCollations    = "COLLATIONS"
	tableFiles         = "FILES"
	catalogVal         = "def"
	tableProfiling     = "PROFILING"
)

type columnInfo struct {
	name  string
	tp    byte
	size  int
	flag  uint
	deflt interface{}
	elems []string
}

func buildColumnInfo(tableName string, col columnInfo) *model.ColumnInfo {
	mCharset := charset.CharsetBin
	mCollation := charset.CharsetBin
	mFlag := mysql.UnsignedFlag
	if col.tp == mysql.TypeVarchar || col.tp == mysql.TypeBlob {
		mCharset = mysql.DefaultCharset
		mCollation = mysql.DefaultCollationName
		mFlag = 0
	}
	fieldType := types.FieldType{
		Charset: mCharset,
		Collate: mCollation,
		Tp:      col.tp,
		Flen:    col.size,
		Flag:    uint(mFlag),
	}
	return &model.ColumnInfo{
		Name:      model.NewCIStr(col.name),
		FieldType: fieldType,
		State:     model.StatePublic,
	}
}

func buildTableMeta(tableName string, cs []columnInfo) *model.TableInfo {
	cols := make([]*model.ColumnInfo, 0, len(cs))
	for _, c := range cs {
		cols = append(cols, buildColumnInfo(tableName, c))
	}
	for i, col := range cols {
		col.Offset = i
	}
	return &model.TableInfo{
		Name:    model.NewCIStr(tableName),
		Columns: cols,
		State:   model.StatePublic,
	}
}

var schemataCols = []columnInfo{
	{"CATALOG_NAME", mysql.TypeVarchar, 512, 0, nil, nil},
	{"SCHEMA_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"DEFAULT_CHARACTER_SET_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"DEFAULT_COLLATION_NAME", mysql.TypeVarchar, 32, 0, nil, nil},
	{"SQL_PATH", mysql.TypeVarchar, 512, 0, nil, nil},
}

var tablesCols = []columnInfo{
	{"TABLE_CATALOG", mysql.TypeVarchar, 512, 0, nil, nil},
	{"TABLE_SCHEMA", mysql.TypeVarchar, 64, 0, nil, nil},
	{"TABLE_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"TABLE_TYPE", mysql.TypeVarchar, 64, 0, nil, nil},
	{"ENGINE", mysql.TypeVarchar, 64, 0, nil, nil},
	{"VERSION", mysql.TypeLonglong, 21, 0, nil, nil},
	{"ROW_FORMAT", mysql.TypeVarchar, 10, 0, nil, nil},
	{"TABLE_ROWS", mysql.TypeLonglong, 21, 0, nil, nil},
	{"AVG_ROW_LENGTH", mysql.TypeLonglong, 21, 0, nil, nil},
	{"DATA_LENGTH", mysql.TypeLonglong, 21, 0, nil, nil},
	{"MAX_DATA_LENGTH", mysql.TypeLonglong, 21, 0, nil, nil},
	{"INDEX_LENGTH", mysql.TypeLonglong, 21, 0, nil, nil},
	{"DATA_FREE", mysql.TypeLonglong, 21, 0, nil, nil},
	{"AUTO_INCREMENT", mysql.TypeLonglong, 21, 0, nil, nil},
	{"CREATE_TIME", mysql.TypeDatetime, 19, 0, nil, nil},
	{"UPDATE_TIME", mysql.TypeDatetime, 19, 0, nil, nil},
	{"CHECK_TIME", mysql.TypeDatetime, 19, 0, nil, nil},
	{"TABLE_COLLATION", mysql.TypeVarchar, 32, 0, nil, nil},
	{"CHECK_SUM", mysql.TypeLonglong, 21, 0, nil, nil},
	{"CREATE_OPTIONS", mysql.TypeVarchar, 255, 0, nil, nil},
	{"TABLE_COMMENT", mysql.TypeVarchar, 2048, 0, nil, nil},
}

var columnsCols = []columnInfo{
	{"TABLE_CATALOG", mysql.TypeVarchar, 512, 0, nil, nil},
	{"TABLE_SCHEMA", mysql.TypeVarchar, 64, 0, nil, nil},
	{"TABLE_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"COLUMN_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"ORIGINAL_POSITION", mysql.TypeLonglong, 64, 0, nil, nil},
	{"COLUMN_DEFAULT", mysql.TypeBlob, 196606, 0, nil, nil},
	{"IS_NULLABLE", mysql.TypeVarchar, 3, 0, nil, nil},
	{"DATA_TYPE", mysql.TypeVarchar, 64, 0, nil, nil},
	{"CHARACTER_MAXIMUM_LENGTH", mysql.TypeLonglong, 21, 0, nil, nil},
	{"CHARACTOR_OCTET_LENGTH", mysql.TypeLonglong, 21, 0, nil, nil},
	{"NUMERIC_PRECISION", mysql.TypeLonglong, 21, 0, nil, nil},
	{"NUMERIC_SCALE", mysql.TypeLonglong, 21, 0, nil, nil},
	{"DATETIME_PRECISION", mysql.TypeLonglong, 21, 0, nil, nil},
	{"CHARACTER_SET_NAME", mysql.TypeVarchar, 32, 0, nil, nil},
	{"COLLATION_NAME", mysql.TypeVarchar, 32, 0, nil, nil},
	{"COLUMN_TYPE", mysql.TypeBlob, 196606, 0, nil, nil},
	{"COLUMN_KEY", mysql.TypeVarchar, 3, 0, nil, nil},
	{"EXTRA", mysql.TypeVarchar, 30, 0, nil, nil},
	{"PRIVILEGES", mysql.TypeVarchar, 80, 0, nil, nil},
	{"COLUMN_COMMENT", mysql.TypeVarchar, 1024, 0, nil, nil},
}

var statisticsCols = []columnInfo{
	{"TABLE_CATALOG", mysql.TypeVarchar, 512, 0, nil, nil},
	{"TABLE_SCHEMA", mysql.TypeVarchar, 64, 0, nil, nil},
	{"TABLE_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"NON_UNIQUE", mysql.TypeVarchar, 1, 0, nil, nil},
	{"INDEX_SCHEMA", mysql.TypeVarchar, 64, 0, nil, nil},
	{"INDEX_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"SEQ_IN_INDEX", mysql.TypeLonglong, 2, 0, nil, nil},
	{"COLUMN_NAME", mysql.TypeVarchar, 21, 0, nil, nil},
	{"COLLATION", mysql.TypeVarchar, 1, 0, nil, nil},
	{"CARDINALITY", mysql.TypeLonglong, 21, 0, nil, nil},
	{"SUB_PART", mysql.TypeLonglong, 3, 0, nil, nil},
	{"PACKED", mysql.TypeVarchar, 10, 0, nil, nil},
	{"NULLABLE", mysql.TypeVarchar, 3, 0, nil, nil},
	{"INDEX_TYPE", mysql.TypeVarchar, 16, 0, nil, nil},
	{"COMMENT", mysql.TypeVarchar, 16, 0, nil, nil},
	{"INDEX_COMMENT", mysql.TypeVarchar, 1024, 0, nil, nil},
}

var profilingCols = []columnInfo{
	{"QUERY_ID", mysql.TypeLong, 20, 0, nil, nil},
	{"SEQ", mysql.TypeLong, 20, 0, nil, nil},
	{"STATE", mysql.TypeVarchar, 30, 0, nil, nil},
	{"DURATION", mysql.TypeNewDecimal, 9, 0, nil, nil},
	{"CPU_USER", mysql.TypeNewDecimal, 9, 0, nil, nil},
	{"CPU_SYSTEM", mysql.TypeNewDecimal, 9, 0, nil, nil},
	{"CONTEXT_VOLUNTARY", mysql.TypeLong, 20, 0, nil, nil},
	{"CONTEXT_INVOLUNTARY", mysql.TypeLong, 20, 0, nil, nil},
	{"BLOCK_OPS_IN", mysql.TypeLong, 20, 0, nil, nil},
	{"BLOCK_OPS_OUT", mysql.TypeLong, 20, 0, nil, nil},
	{"MESSAGES_SENT", mysql.TypeLong, 20, 0, nil, nil},
	{"MESSAGES_RECEIVED", mysql.TypeLong, 20, 0, nil, nil},
	{"PAGE_FAULTS_MAJOR", mysql.TypeLong, 20, 0, nil, nil},
	{"PAGE_FAULTS_MINOR", mysql.TypeLong, 20, 0, nil, nil},
	{"SWAPS", mysql.TypeLong, 20, 0, nil, nil},
	{"SOURCE_FUNCTION", mysql.TypeVarchar, 30, 0, nil, nil},
	{"SOURCE_FILE", mysql.TypeVarchar, 20, 0, nil, nil},
	{"SOURCE_LINE", mysql.TypeLong, 20, 0, nil, nil},
}

var charsetCols = []columnInfo{
	{"CHARACTER_SET_NAME", mysql.TypeVarchar, 32, 0, nil, nil},
	{"DEFAULT_COLLATE_NAME", mysql.TypeVarchar, 32, 0, nil, nil},
	{"DESCRIPTION", mysql.TypeVarchar, 60, 0, nil, nil},
	{"MAXLEN", mysql.TypeLonglong, 3, 0, nil, nil},
}

var collationsCols = []columnInfo{
	{"COLLATION_NAME", mysql.TypeVarchar, 32, 0, nil, nil},
	{"CHARACTER_SET_NAME", mysql.TypeVarchar, 32, 0, nil, nil},
	{"ID", mysql.TypeLonglong, 11, 0, nil, nil},
	{"IS_DEFAULT", mysql.TypeVarchar, 3, 0, nil, nil},
	{"IS_COMPILED", mysql.TypeVarchar, 3, 0, nil, nil},
	{"SORTLEN", mysql.TypeLonglong, 3, 0, nil, nil},
}

func dataForCharacterSets() (records [][]types.Datum) {
	records = append(records,
		types.MakeDatums("ascii", "ascii_general_ci", "US ASCII", 1),
		types.MakeDatums("binary", "binary", "Binary pseudo charset", 1),
		types.MakeDatums("latin1", "latin1_swedish_ci", "cp1252 West European", 1),
		types.MakeDatums("utf8", "utf8_general_ci", "UTF-8 Unicode", 3),
		types.MakeDatums("utf8mb4", "utf8mb4_general_ci", "UTF-8 Unicode", 4),
	)
	return records
}

func dataForColltions() (records [][]types.Datum) {
	records = append(records,
		types.MakeDatums("ascii_general_ci", "ascii", 1, "Yes", "Yes", 1),
		types.MakeDatums("binary", "binary", 2, "Yes", "Yes", 1),
		types.MakeDatums("latin1_swedish_ci", "latin1", 3, "Yes", "Yes", 1),
		types.MakeDatums("utf8_general_ci", "utf8", 4, "Yes", "Yes", 1),
		types.MakeDatums("utf8mb4_general_ci", "utf8mb4", 5, "Yes", "Yes", 1),
	)
	return records
}

var filesCols = []columnInfo{
	{"FILE_ID", mysql.TypeLonglong, 4, 0, nil, nil},
	{"FILE_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"TABLESPACE_NAME", mysql.TypeVarchar, 20, 0, nil, nil},
	{"TABLE_CATALOG", mysql.TypeVarchar, 64, 0, nil, nil},
	{"TABLE_SCHEMA", mysql.TypeVarchar, 64, 0, nil, nil},
	{"TABLE_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"LOGFILE_GROUP_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"LOGFILE_GROUP_NUMBER", mysql.TypeLonglong, 32, 0, nil, nil},
	{"ENGINE", mysql.TypeVarchar, 64, 0, nil, nil},
	{"FULLTEXT_KEYS", mysql.TypeVarchar, 64, 0, nil, nil},
	{"DELETED_ROWS", mysql.TypeLonglong, 4, 0, nil, nil},
	{"UPDATE_COUNT", mysql.TypeLonglong, 4, 0, nil, nil},
	{"FREE_EXTENTS", mysql.TypeLonglong, 4, 0, nil, nil},
	{"TOTAL_EXTENTS", mysql.TypeLonglong, 4, 0, nil, nil},
	{"EXTENT_SIZE", mysql.TypeLonglong, 4, 0, nil, nil},
	{"INITIAL_SIZE", mysql.TypeLonglong, 21, 0, nil, nil},
	{"MAXIMUM_SIZE", mysql.TypeLonglong, 21, 0, nil, nil},
	{"AUTOEXTEND_SIZE", mysql.TypeLonglong, 21, 0, nil, nil},
	{"CREATION_TIME", mysql.TypeDatetime, -1, 0, nil, nil},
	{"LAST_UPDATE_TIME", mysql.TypeDatetime, -1, 0, nil, nil},
	{"LAST_ACCESS_TIME", mysql.TypeDatetime, -1, 0, nil, nil},
	{"RECOVER_TIME", mysql.TypeLonglong, 4, 0, nil, nil},
	{"TRANSACTION_COUNTER", mysql.TypeLonglong, 4, 0, nil, nil},
	{"VERSION", mysql.TypeLonglong, 21, 0, nil, nil},
	{"ROW_FORMAT", mysql.TypeVarchar, 21, 0, nil, nil},
	{"TABLE_ROWS", mysql.TypeLonglong, 21, 0, nil, nil},
	{"AVG_ROW_LENGTH", mysql.TypeLonglong, 21, 0, nil, nil},
	{"DATA_FREE", mysql.TypeLonglong, 21, 0, nil, nil},
	{"CREATE_TIME", mysql.TypeDatetime, -1, 0, nil, nil},
	{"UPDATE_TIME", mysql.TypeDatetime, -1, 0, nil, nil},
	{"CHECK_TIME", mysql.TypeDatetime, -1, 0, nil, nil},
	{"CHECKSUM", mysql.TypeLonglong, 21, 0, nil, nil},
	{"STATUS", mysql.TypeVarchar, 20, 0, nil, nil},
	{"EXTRA", mysql.TypeVarchar, 255, 0, nil, nil},
}

func dataForSchemata(schemas []string) [][]types.Datum {
	sort.Strings(schemas)
	rows := [][]types.Datum{}
	for _, schema := range schemas {
		record := types.MakeDatums(
			catalogVal,                 // CATALOG_NAME
			schema,                     // SCHEMA_NAME
			mysql.DefaultCharset,       // DEFAULT_CHARACTER_SET_NAME
			mysql.DefaultCollationName, // DEFAULT_COLLATION_NAME
			nil,
		)
		rows = append(rows, record)
	}
	return rows
}

func dataForTables(schemas []*model.DBInfo) [][]types.Datum {
	rows := [][]types.Datum{}
	for _, schema := range schemas {
		for _, table := range schema.Tables {
			record := types.MakeDatums(
				catalogVal,          // TABLE_CATALOG
				schema.Name.O,       // TABLE_SCHEMA
				table.Name.O,        // TABLE_NAME
				"BASE_TABLE",        // TABLE_TYPE
				"InnoDB",            // ENGINE
				uint64(10),          // VERSION
				"Compact",           // ROW_FORMAT
				uint64(0),           // TABLE_ROWS
				uint64(0),           // AVG_ROW_LENGTH
				uint64(16384),       // DATA_LENGTH
				uint64(0),           // MAX_DATA_LENGTH
				uint64(0),           // INDEX_LENGTH
				uint64(0),           // DATA_FREE
				nil,                 // AUTO_INCREMENT
				nil,                 // CREATE_TIME
				nil,                 // UPDATE_TIME
				nil,                 // CHECK_TIME
				"latin1_swedish_ci", // TABLE_COLLATION
				nil,                 // CHECKSUM
				"",                  // CREATE_OPTIONS
				"",                  // TABLE_COMMENT
			)
			rows = append(rows, record)
		}
	}
	return rows
}

func dataForColumns(schemas []*model.DBInfo) [][]types.Datum {
	rows := [][]types.Datum{}
	for _, schema := range schemas {
		for _, table := range schema.Tables {
			rs := dataForColumnsInTable(schema, table)
			for _, r := range rs {
				rows = append(rows, r)
			}
		}
	}
	return rows
}

func dataForColumnsInTable(schema *model.DBInfo, table *model.TableInfo) [][]types.Datum {
	rows := [][]types.Datum{}
	for i, col := range table.Columns {
		colLen := col.Flen
		if colLen == types.UnspecifiedLength {
			colLen = mysql.GetDefaultFieldLength(col.Tp)
		}
		decimal := col.Decimal
		if decimal == types.UnspecifiedLength {
			decimal = 0
		}
		columnType := col.FieldType.CompactStr()
		columnDesc := column.NewColDesc(&column.Col{ColumnInfo: *col})
		var columnDefault interface{}
		if columnDesc.DefaultValue != nil {
			columnDefault = fmt.Sprintf("%v", columnDesc.DefaultValue)
		}
		record := types.MakeDatums(
			catalogVal,                           // TABLE_CATALOG
			schema.Name.O,                        // TABLE_SCHEMA
			table.Name.O,                         // TABLE_NAME
			col.Name.O,                           // COLUMN_NAME
			i+1,                                  // ORIGINAL_POSITION
			columnDefault,                        // COLUMN_DEFAULT
			columnDesc.Null,                      // IS_NULLABLE
			types.TypeToStr(col.Tp, col.Charset), // DATA_TYPE
			colLen,                            // CHARACTER_MAXIMUM_LENGTH
			colLen,                            // CHARACTOR_OCTET_LENGTH
			decimal,                           // NUMERIC_PRECISION
			0,                                 // NUMERIC_SCALE
			0,                                 // DATETIME_PRECISION
			col.Charset,                       // CHARACTER_SET_NAME
			col.Collate,                       // COLLATION_NAME
			columnType,                        // COLUMN_TYPE
			columnDesc.Key,                    // COLUMN_KEY
			columnDesc.Extra,                  // EXTRA
			"select,insert,update,references", // PRIVILEGES
			"", // COLUMN_COMMENT
		)
		rows = append(rows, record)
	}
	return rows
}

func dataForStatistics(schemas []*model.DBInfo) [][]types.Datum {
	rows := [][]types.Datum{}
	for _, schema := range schemas {
		for _, table := range schema.Tables {
			rs := dataForStatisticsInTable(schema, table)
			for _, r := range rs {
				rows = append(rows, r)
			}
		}
	}
	return rows
}

func dataForStatisticsInTable(schema *model.DBInfo, table *model.TableInfo) [][]types.Datum {
	rows := [][]types.Datum{}
	if table.PKIsHandle {
		for _, col := range table.Columns {
			if mysql.HasPriKeyFlag(col.Flag) {
				record := types.MakeDatums(
					catalogVal,    // TABLE_CATALOG
					schema.Name.O, // TABLE_SCHEMA
					table.Name.O,  // TABLE_NAME
					"0",           // NON_UNIQUE
					schema.Name.O, // INDEX_SCHEMA
					"PRIMARY",     // INDEX_NAME
					1,             // SEQ_IN_INDEX
					col.Name.O,    // COLUMN_NAME
					"A",           // COLLATION
					0,             // CARDINALITY
					nil,           // SUB_PART
					nil,           // PACKED
					"",            // NULLABLE
					"BTREE",       // INDEX_TYPE
					"",            // COMMENT
					"",            // INDEX_COMMENT
				)
				rows = append(rows, record)
			}
		}
	}
	nameToCol := make(map[string]*model.ColumnInfo, len(table.Columns))
	for _, c := range table.Columns {
		nameToCol[c.Name.L] = c
	}
	for _, index := range table.Indices {
		nonUnique := "1"
		if index.Unique {
			nonUnique = "0"
		}
		for i, key := range index.Columns {
			col := nameToCol[key.Name.L]
			nullable := "YES"
			if mysql.HasNotNullFlag(col.Flag) {
				nullable = ""
			}
			record := types.MakeDatums(
				catalogVal,    // TABLE_CATALOG
				schema.Name.O, // TABLE_SCHEMA
				table.Name.O,  // TABLE_NAME
				nonUnique,     // NON_UNIQUE
				schema.Name.O, // INDEX_SCHEMA
				index.Name.O,  // INDEX_NAME
				i+1,           // SEQ_IN_INDEX
				key.Name.O,    // COLUMN_NAME
				"A",           // COLLATION
				0,             // CARDINALITY
				nil,           // SUB_PART
				nil,           // PACKED
				nullable,      // NULLABLE
				"BTREE",       // INDEX_TYPE
				"",            // COMMENT
				"",            // INDEX_COMMENT
			)
			rows = append(rows, record)
		}
	}
	return rows
}

var tableNameToColumns = map[string]([]columnInfo){
	tableSchemata:      schemataCols,
	tableTables:        tablesCols,
	tableColumns:       columnsCols,
	tableStatistics:    statisticsCols,
	tableCharacterSets: charsetCols,
	tableCollations:    collationsCols,
	tableFiles:         filesCols,
	tableProfiling:     profilingCols,
}

func createMemoryTable(meta *model.TableInfo, alloc autoid.Allocator) (table.Table, error) {
	tbl, _ := tables.MemoryTableFromMeta(alloc, meta)
	return tbl, nil
}
