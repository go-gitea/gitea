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

package column

import (
	"strings"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/util/types"
)

// Col provides meta data describing a table column.
type Col struct {
	model.ColumnInfo
}

// PrimaryKeyName defines primary key name.
const PrimaryKeyName = "PRIMARY"

// IndexedCol defines an index with info.
type IndexedCol struct {
	model.IndexInfo
	X kv.Index
}

// String implements fmt.Stringer interface.
func (c *Col) String() string {
	ans := []string{c.Name.O, types.TypeToStr(c.Tp, c.Charset)}
	if mysql.HasAutoIncrementFlag(c.Flag) {
		ans = append(ans, "AUTO_INCREMENT")
	}
	if mysql.HasNotNullFlag(c.Flag) {
		ans = append(ans, "NOT NULL")
	}
	return strings.Join(ans, " ")
}

// FindCol finds column in cols by name.
func FindCol(cols []*Col, name string) *Col {
	for _, col := range cols {
		if strings.EqualFold(col.Name.O, name) {
			return col
		}
	}
	return nil
}

// FindCols finds columns in cols by names.
func FindCols(cols []*Col, names []string) ([]*Col, error) {
	var rcols []*Col
	for _, name := range names {
		col := FindCol(cols, name)
		if col != nil {
			rcols = append(rcols, col)
		} else {
			return nil, errors.Errorf("unknown column %s", name)
		}
	}

	return rcols, nil
}

// FindOnUpdateCols finds columns which have OnUpdateNow flag.
func FindOnUpdateCols(cols []*Col) []*Col {
	var rcols []*Col
	for _, col := range cols {
		if mysql.HasOnUpdateNowFlag(col.Flag) {
			rcols = append(rcols, col)
		}
	}

	return rcols
}

// CastValues casts values based on columns type.
func CastValues(ctx context.Context, rec []types.Datum, cols []*Col) (err error) {
	for _, c := range cols {
		var converted types.Datum
		converted, err = rec[c.Offset].ConvertTo(&c.FieldType)
		if err != nil {
			return errors.Trace(err)
		}
		rec[c.Offset] = converted
	}
	return nil
}

// ColDesc describes column information like MySQL desc and show columns do.
type ColDesc struct {
	Field        string
	Type         string
	Collation    string
	Null         string
	Key          string
	DefaultValue interface{}
	Extra        string
	Privileges   string
	Comment      string
}

const defaultPrivileges string = "select,insert,update,references"

// GetTypeDesc gets the description for column type.
func (c *Col) GetTypeDesc() string {
	desc := c.FieldType.CompactStr()
	if mysql.HasUnsignedFlag(c.Flag) {
		desc += " UNSIGNED"
	}
	return desc
}

// NewColDesc returns a new ColDesc for a column.
func NewColDesc(col *Col) *ColDesc {
	// TODO: if we have no primary key and a unique index which's columns are all not null
	// we will set these columns' flag as PriKeyFlag
	// see https://dev.mysql.com/doc/refman/5.7/en/show-columns.html
	// create table
	name := col.Name
	nullFlag := "YES"
	if mysql.HasNotNullFlag(col.Flag) {
		nullFlag = "NO"
	}
	keyFlag := ""
	if mysql.HasPriKeyFlag(col.Flag) {
		keyFlag = "PRI"
	} else if mysql.HasUniKeyFlag(col.Flag) {
		keyFlag = "UNI"
	} else if mysql.HasMultipleKeyFlag(col.Flag) {
		keyFlag = "MUL"
	}
	var defaultValue interface{}
	if !mysql.HasNoDefaultValueFlag(col.Flag) {
		defaultValue = col.DefaultValue
	}

	extra := ""
	if mysql.HasAutoIncrementFlag(col.Flag) {
		extra = "auto_increment"
	} else if mysql.HasOnUpdateNowFlag(col.Flag) {
		extra = "on update CURRENT_TIMESTAMP"
	}

	return &ColDesc{
		Field:        name.O,
		Type:         col.GetTypeDesc(),
		Collation:    col.Collate,
		Null:         nullFlag,
		Key:          keyFlag,
		DefaultValue: defaultValue,
		Extra:        extra,
		Privileges:   defaultPrivileges,
		Comment:      "",
	}
}

// ColDescFieldNames returns the fields name in result set for desc and show columns.
func ColDescFieldNames(full bool) []string {
	if full {
		return []string{"Field", "Type", "Collation", "Null", "Key", "Default", "Extra", "Privileges", "Comment"}
	}
	return []string{"Field", "Type", "Null", "Key", "Default", "Extra"}
}

// CheckOnce checks if there are duplicated column names in cols.
func CheckOnce(cols []*Col) error {
	m := map[string]struct{}{}
	for _, col := range cols {
		name := col.Name
		_, ok := m[name.L]
		if ok {
			return errors.Errorf("column specified twice - %s", name)
		}

		m[name.L] = struct{}{}
	}

	return nil
}

// CheckNotNull checks if nil value set to a column with NotNull flag is set.
func (c *Col) CheckNotNull(data types.Datum) error {
	if mysql.HasNotNullFlag(c.Flag) && data.Kind() == types.KindNull {
		return errors.Errorf("Column %s can't be null.", c.Name)
	}
	return nil
}

// IsPKHandleColumn checks if the column is primary key handle column.
func (c *Col) IsPKHandleColumn(tbInfo *model.TableInfo) bool {
	return mysql.HasPriKeyFlag(c.Flag) && tbInfo.PKIsHandle
}

// CheckNotNull checks if row has nil value set to a column with NotNull flag set.
func CheckNotNull(cols []*Col, row []types.Datum) error {
	for _, c := range cols {
		if err := c.CheckNotNull(row[c.Offset]); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// FetchValues fetches indexed values from a row.
func (idx *IndexedCol) FetchValues(r []types.Datum) ([]types.Datum, error) {
	vals := make([]types.Datum, len(idx.Columns))
	for i, ic := range idx.Columns {
		if ic.Offset < 0 || ic.Offset > len(r) {
			return nil, errors.New("Index column offset out of bound")
		}
		vals[i] = r[ic.Offset]
	}
	return vals, nil
}
