// Copyright 2015 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tidb

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-xorm/core"
)

type tidb struct {
	core.Base
}

func (db *tidb) Init(d *core.DB, uri *core.Uri, drivername, dataSourceName string) error {
	return db.Base.Init(d, db, uri, drivername, dataSourceName)
}

func (db *tidb) SqlType(c *core.Column) string {
	var res string
	switch t := c.SQLType.Name; t {
	case core.Bool:
		res = core.Bool
	case core.Serial:
		c.IsAutoIncrement = true
		c.IsPrimaryKey = true
		c.Nullable = false
		res = core.Int
	case core.BigSerial:
		c.IsAutoIncrement = true
		c.IsPrimaryKey = true
		c.Nullable = false
		res = core.BigInt
	case core.Bytea:
		res = core.Blob
	case core.TimeStampz:
		res = core.Char
		c.Length = 64
	case core.Enum: //mysql enum
		res = core.Enum
		res += "("
		opts := ""
		for v, _ := range c.EnumOptions {
			opts += fmt.Sprintf(",'%v'", v)
		}
		res += strings.TrimLeft(opts, ",")
		res += ")"
	case core.Set: //mysql set
		res = core.Set
		res += "("
		opts := ""
		for v, _ := range c.SetOptions {
			opts += fmt.Sprintf(",'%v'", v)
		}
		res += strings.TrimLeft(opts, ",")
		res += ")"
	case core.NVarchar:
		res = core.Varchar
	case core.Uuid:
		res = core.Varchar
		c.Length = 40
	case core.Json:
		res = core.Text
	default:
		res = t
	}

	var hasLen1 bool = (c.Length > 0)
	var hasLen2 bool = (c.Length2 > 0)

	if res == core.BigInt && !hasLen1 && !hasLen2 {
		c.Length = 20
		hasLen1 = true
	}

	if hasLen2 {
		res += "(" + strconv.Itoa(c.Length) + "," + strconv.Itoa(c.Length2) + ")"
	} else if hasLen1 {
		res += "(" + strconv.Itoa(c.Length) + ")"
	}
	return res
}

func (db *tidb) SupportInsertMany() bool {
	return true
}

func (db *tidb) IsReserved(name string) bool {
	return false
}

func (db *tidb) Quote(name string) string {
	return "`" + name + "`"
}

func (db *tidb) QuoteStr() string {
	return "`"
}

func (db *tidb) SupportEngine() bool {
	return false
}

func (db *tidb) AutoIncrStr() string {
	return "AUTO_INCREMENT"
}

func (db *tidb) SupportCharset() bool {
	return false
}

func (db *tidb) IndexOnTable() bool {
	return true
}

func (db *tidb) IndexCheckSql(tableName, idxName string) (string, []interface{}) {
	args := []interface{}{db.DbName, tableName, idxName}
	sql := "SELECT `INDEX_NAME` FROM `INFORMATION_SCHEMA`.`STATISTICS`"
	sql += " WHERE `TABLE_SCHEMA` = ? AND `TABLE_NAME` = ? AND `INDEX_NAME`=?"
	return sql, args
}

func (db *tidb) TableCheckSql(tableName string) (string, []interface{}) {
	args := []interface{}{db.DbName, tableName}
	sql := "SELECT `TABLE_NAME` from `INFORMATION_SCHEMA`.`TABLES` WHERE `TABLE_SCHEMA`=? and `TABLE_NAME`=?"
	return sql, args
}

func (db *tidb) GetColumns(tableName string) ([]string, map[string]*core.Column, error) {
	args := []interface{}{db.DbName, tableName}
	s := "SELECT `COLUMN_NAME`, `IS_NULLABLE`, `COLUMN_DEFAULT`, `COLUMN_TYPE`," +
		" `COLUMN_KEY`, `EXTRA` FROM `INFORMATION_SCHEMA`.`COLUMNS` WHERE `TABLE_SCHEMA` = ? AND `TABLE_NAME` = ?"

	rows, err := db.DB().Query(s, args...)
	db.LogSQL(s, args)

	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	cols := make(map[string]*core.Column)
	colSeq := make([]string, 0)
	for rows.Next() {
		col := new(core.Column)
		col.Indexes = make(map[string]int)

		var columnName, isNullable, colType, colKey, extra string
		var colDefault *string
		err = rows.Scan(&columnName, &isNullable, &colDefault, &colType, &colKey, &extra)
		if err != nil {
			return nil, nil, err
		}
		col.Name = strings.Trim(columnName, "` ")
		if "YES" == isNullable {
			col.Nullable = true
		}

		if colDefault != nil {
			col.Default = *colDefault
			if col.Default == "" {
				col.DefaultIsEmpty = true
			}
		}

		cts := strings.Split(colType, "(")
		colName := cts[0]
		colType = strings.ToUpper(colName)
		var len1, len2 int
		if len(cts) == 2 {
			idx := strings.Index(cts[1], ")")
			if colType == core.Enum && cts[1][0] == '\'' { //enum
				options := strings.Split(cts[1][0:idx], ",")
				col.EnumOptions = make(map[string]int)
				for k, v := range options {
					v = strings.TrimSpace(v)
					v = strings.Trim(v, "'")
					col.EnumOptions[v] = k
				}
			} else if colType == core.Set && cts[1][0] == '\'' {
				options := strings.Split(cts[1][0:idx], ",")
				col.SetOptions = make(map[string]int)
				for k, v := range options {
					v = strings.TrimSpace(v)
					v = strings.Trim(v, "'")
					col.SetOptions[v] = k
				}
			} else {
				lens := strings.Split(cts[1][0:idx], ",")
				len1, err = strconv.Atoi(strings.TrimSpace(lens[0]))
				if err != nil {
					return nil, nil, err
				}
				if len(lens) == 2 {
					len2, err = strconv.Atoi(lens[1])
					if err != nil {
						return nil, nil, err
					}
				}
			}
		}
		if colType == "FLOAT UNSIGNED" {
			colType = "FLOAT"
		}
		col.Length = len1
		col.Length2 = len2
		if _, ok := core.SqlTypes[colType]; ok {
			col.SQLType = core.SQLType{colType, len1, len2}
		} else {
			return nil, nil, errors.New(fmt.Sprintf("unkonw colType %v", colType))
		}

		if colKey == "PRI" {
			col.IsPrimaryKey = true
		}
		if colKey == "UNI" {
			//col.is
		}

		if extra == "auto_increment" {
			col.IsAutoIncrement = true
		}

		if col.SQLType.IsText() || col.SQLType.IsTime() {
			if col.Default != "" {
				col.Default = "'" + col.Default + "'"
			} else {
				if col.DefaultIsEmpty {
					col.Default = "''"
				}
			}
		}
		cols[col.Name] = col
		colSeq = append(colSeq, col.Name)
	}
	return colSeq, cols, nil
}

func (db *tidb) GetTables() ([]*core.Table, error) {
	args := []interface{}{db.DbName}
	s := "SELECT `TABLE_NAME`, `ENGINE`, `TABLE_ROWS`, `AUTO_INCREMENT` from " +
		"`INFORMATION_SCHEMA`.`TABLES` WHERE `TABLE_SCHEMA`=? AND (`ENGINE`='MyISAM' OR `ENGINE` = 'InnoDB')"

	rows, err := db.DB().Query(s, args...)
	db.LogSQL(s, args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make([]*core.Table, 0)
	for rows.Next() {
		table := core.NewEmptyTable()
		var name, engine, tableRows string
		var autoIncr *string
		err = rows.Scan(&name, &engine, &tableRows, &autoIncr)
		if err != nil {
			return nil, err
		}

		table.Name = name
		table.StoreEngine = engine
		tables = append(tables, table)
	}
	return tables, nil
}

func (db *tidb) GetIndexes(tableName string) (map[string]*core.Index, error) {
	args := []interface{}{db.DbName, tableName}
	s := "SELECT `INDEX_NAME`, `NON_UNIQUE`, `COLUMN_NAME` FROM `INFORMATION_SCHEMA`.`STATISTICS` WHERE `TABLE_SCHEMA` = ? AND `TABLE_NAME` = ?"

	rows, err := db.DB().Query(s, args...)
	db.LogSQL(s, args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexes := make(map[string]*core.Index, 0)
	for rows.Next() {
		var indexType int
		var indexName, colName, nonUnique string
		err = rows.Scan(&indexName, &nonUnique, &colName)
		if err != nil {
			return nil, err
		}

		if indexName == "PRIMARY" {
			continue
		}

		if "YES" == nonUnique || nonUnique == "1" {
			indexType = core.IndexType
		} else {
			indexType = core.UniqueType
		}

		colName = strings.Trim(colName, "` ")
		var isRegular bool
		if strings.HasPrefix(indexName, "IDX_"+tableName) || strings.HasPrefix(indexName, "UQE_"+tableName) {
			indexName = indexName[5+len(tableName) : len(indexName)]
			isRegular = true
		}

		var index *core.Index
		var ok bool
		if index, ok = indexes[indexName]; !ok {
			index = new(core.Index)
			index.IsRegular = isRegular
			index.Type = indexType
			index.Name = indexName
			indexes[indexName] = index
		}
		index.AddColumn(colName)
	}
	return indexes, nil
}

func (db *tidb) Filters() []core.Filter {
	return []core.Filter{&core.IdFilter{}}
}
