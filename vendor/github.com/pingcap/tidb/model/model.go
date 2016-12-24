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

package model

import (
	"strings"

	"github.com/pingcap/tidb/util/types"
)

// SchemaState is the state for schema elements.
type SchemaState byte

const (
	// StateNone means this schema element is absent and can't be used.
	StateNone SchemaState = iota
	// StateDeleteOnly means we can only delete items for this schema element.
	StateDeleteOnly
	// StateWriteOnly means we can use any write operation on this schema element,
	// but outer can't read the changed data.
	StateWriteOnly
	// StateWriteReorganization means we are re-organizating whole data after write only state.
	StateWriteReorganization
	// StateDeleteReorganization means we are re-organizating whole data after delete only state.
	StateDeleteReorganization
	// StatePublic means this schema element is ok for all write and read operations.
	StatePublic
)

// String implements fmt.Stringer interface.
func (s SchemaState) String() string {
	switch s {
	case StateDeleteOnly:
		return "delete only"
	case StateWriteOnly:
		return "write only"
	case StateWriteReorganization:
		return "write reorganization"
	case StateDeleteReorganization:
		return "delete reorganization"
	case StatePublic:
		return "public"
	default:
		return "none"
	}
}

// ColumnInfo provides meta data describing of a table column.
type ColumnInfo struct {
	ID              int64       `json:"id"`
	Name            CIStr       `json:"name"`
	Offset          int         `json:"offset"`
	DefaultValue    interface{} `json:"default"`
	types.FieldType `json:"type"`
	State           SchemaState `json:"state"`
}

// Clone clones ColumnInfo.
func (c *ColumnInfo) Clone() *ColumnInfo {
	nc := *c
	return &nc
}

// TableInfo provides meta data describing a DB table.
type TableInfo struct {
	ID      int64  `json:"id"`
	Name    CIStr  `json:"name"`
	Charset string `json:"charset"`
	Collate string `json:"collate"`
	// Columns are listed in the order in which they appear in the schema.
	Columns    []*ColumnInfo `json:"cols"`
	Indices    []*IndexInfo  `json:"index_info"`
	State      SchemaState   `json:"state"`
	PKIsHandle bool          `json:"pk_is_handle"`
	Comment    string        `json:"comment"`
}

// Clone clones TableInfo.
func (t *TableInfo) Clone() *TableInfo {
	nt := *t
	nt.Columns = make([]*ColumnInfo, len(t.Columns))
	nt.Indices = make([]*IndexInfo, len(t.Indices))

	for i := range t.Columns {
		nt.Columns[i] = t.Columns[i].Clone()
	}

	for i := range t.Indices {
		nt.Indices[i] = t.Indices[i].Clone()
	}
	return &nt
}

// IndexColumn provides index column info.
type IndexColumn struct {
	Name   CIStr `json:"name"`   // Index name
	Offset int   `json:"offset"` // Index offset
	Length int   `json:"length"` // Index length
}

// Clone clones IndexColumn.
func (i *IndexColumn) Clone() *IndexColumn {
	ni := *i
	return &ni
}

// IndexType is the type of index
type IndexType int

// String implements Stringer interface.
func (t IndexType) String() string {
	switch t {
	case IndexTypeBtree:
		return "BTREE"
	case IndexTypeHash:
		return "HASH"
	}
	return ""
}

// IndexTypes
const (
	IndexTypeBtree IndexType = iota + 1
	IndexTypeHash
)

// IndexInfo provides meta data describing a DB index.
// It corresponds to the statement `CREATE INDEX Name ON Table (Column);`
// See: https://dev.mysql.com/doc/refman/5.7/en/create-index.html
type IndexInfo struct {
	ID      int64          `json:"id"`
	Name    CIStr          `json:"idx_name"`   // Index name.
	Table   CIStr          `json:"tbl_name"`   // Table name.
	Columns []*IndexColumn `json:"idx_cols"`   // Index columns.
	Unique  bool           `json:"is_unique"`  // Whether the index is unique.
	Primary bool           `json:"is_primary"` // Whether the index is primary key.
	State   SchemaState    `json:"state"`
	Comment string         `json:"comment"`    // Comment
	Tp      IndexType      `json:"index_type"` // Index type: Btree or Hash
}

// Clone clones IndexInfo.
func (index *IndexInfo) Clone() *IndexInfo {
	ni := *index
	ni.Columns = make([]*IndexColumn, len(index.Columns))
	for i := range index.Columns {
		ni.Columns[i] = index.Columns[i].Clone()
	}
	return &ni
}

// DBInfo provides meta data describing a DB.
type DBInfo struct {
	ID      int64        `json:"id"`      // Database ID
	Name    CIStr        `json:"db_name"` // DB name.
	Charset string       `json:"charset"`
	Collate string       `json:"collate"`
	Tables  []*TableInfo `json:"-"` // Tables in the DB.
	State   SchemaState  `json:"state"`
}

// Clone clones DBInfo.
func (db *DBInfo) Clone() *DBInfo {
	newInfo := *db
	newInfo.Tables = make([]*TableInfo, len(db.Tables))
	for i := range db.Tables {
		newInfo.Tables[i] = db.Tables[i].Clone()
	}
	return &newInfo
}

// CIStr is case insensitve string.
type CIStr struct {
	O string `json:"O"` // Original string.
	L string `json:"L"` // Lower case string.
}

// String implements fmt.Stringer interface.
func (cis CIStr) String() string {
	return cis.O
}

// NewCIStr creates a new CIStr.
func NewCIStr(s string) (cs CIStr) {
	cs.O = s
	cs.L = strings.ToLower(s)
	return
}
