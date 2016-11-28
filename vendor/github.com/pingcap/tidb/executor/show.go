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

package executor

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/column"
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/infoschema"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/privilege"
	"github.com/pingcap/tidb/sessionctx/variable"
	"github.com/pingcap/tidb/table"
	"github.com/pingcap/tidb/util/charset"
	"github.com/pingcap/tidb/util/types"
)

// ShowExec represents a show executor.
type ShowExec struct {
	Tp     ast.ShowStmtType // Databases/Tables/Columns/....
	DBName model.CIStr
	Table  *ast.TableName  // Used for showing columns.
	Column *ast.ColumnName // Used for `desc table column`.
	Flag   int             // Some flag parsed from sql, such as FULL.
	Full   bool
	User   string // Used for show grants.

	// Used by show variables
	GlobalScope bool

	fields []*ast.ResultField
	ctx    context.Context
	is     infoschema.InfoSchema

	fetched bool
	rows    []*Row
	cursor  int
}

// Fields implements Executor Fields interface.
func (e *ShowExec) Fields() []*ast.ResultField {
	return e.fields
}

// Next implements Execution Next interface.
func (e *ShowExec) Next() (*Row, error) {
	if e.rows == nil {
		err := e.fetchAll()
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	if e.cursor >= len(e.rows) {
		return nil, nil
	}
	row := e.rows[e.cursor]
	for i, field := range e.fields {
		field.Expr.SetValue(row.Data[i].GetValue())
	}
	e.cursor++
	return row, nil
}

func (e *ShowExec) fetchAll() error {
	switch e.Tp {
	case ast.ShowCharset:
		return e.fetchShowCharset()
	case ast.ShowCollation:
		return e.fetchShowCollation()
	case ast.ShowColumns:
		return e.fetchShowColumns()
	case ast.ShowCreateTable:
		return e.fetchShowCreateTable()
	case ast.ShowDatabases:
		return e.fetchShowDatabases()
	case ast.ShowEngines:
		return e.fetchShowEngines()
	case ast.ShowGrants:
		return e.fetchShowGrants()
	case ast.ShowIndex:
		return e.fetchShowIndex()
	case ast.ShowProcedureStatus:
		return e.fetchShowProcedureStatus()
	case ast.ShowStatus:
		return e.fetchShowStatus()
	case ast.ShowTables:
		return e.fetchShowTables()
	case ast.ShowTableStatus:
		return e.fetchShowTableStatus()
	case ast.ShowTriggers:
		return e.fetchShowTriggers()
	case ast.ShowVariables:
		return e.fetchShowVariables()
	case ast.ShowWarnings:
		// empty result
	}
	return nil
}

func (e *ShowExec) fetchShowEngines() error {
	row := &Row{
		Data: types.MakeDatums(
			"InnoDB",
			"DEFAULT",
			"Supports transactions, row-level locking, and foreign keys",
			"YES",
			"YES",
			"YES",
		),
	}
	e.rows = append(e.rows, row)
	return nil
}

func (e *ShowExec) fetchShowDatabases() error {
	dbs := e.is.AllSchemaNames()
	// TODO: let information_schema be the first database
	sort.Strings(dbs)
	for _, d := range dbs {
		e.rows = append(e.rows, &Row{Data: types.MakeDatums(d)})
	}
	return nil
}

func (e *ShowExec) fetchShowTables() error {
	if !e.is.SchemaExists(e.DBName) {
		return errors.Errorf("Can not find DB: %s", e.DBName)
	}
	// sort for tables
	var tableNames []string
	for _, v := range e.is.SchemaTables(e.DBName) {
		tableNames = append(tableNames, v.Meta().Name.L)
	}
	sort.Strings(tableNames)
	for _, v := range tableNames {
		data := types.MakeDatums(v)
		if e.Full {
			// TODO: support "VIEW" later if we have supported view feature.
			// now, just use "BASE TABLE".
			data = append(data, types.NewDatum("BASE TABLE"))
		}
		e.rows = append(e.rows, &Row{Data: data})
	}
	return nil
}

func (e *ShowExec) fetchShowTableStatus() error {
	if !e.is.SchemaExists(e.DBName) {
		return errors.Errorf("Can not find DB: %s", e.DBName)
	}

	// sort for tables
	var tableNames []string
	for _, v := range e.is.SchemaTables(e.DBName) {
		tableNames = append(tableNames, v.Meta().Name.L)
	}
	sort.Strings(tableNames)

	for _, v := range tableNames {
		now := mysql.CurrentTime(mysql.TypeDatetime)
		data := types.MakeDatums(v, "InnoDB", "10", "Compact", 100, 100, 100, 100, 100, 100, 100,
			now, now, now, "utf8_general_ci", "", "", "")
		e.rows = append(e.rows, &Row{Data: data})
	}
	return nil
}

func (e *ShowExec) fetchShowColumns() error {
	tb, err := e.getTable()
	if err != nil {
		return errors.Trace(err)
	}
	cols := tb.Cols()
	for _, col := range cols {
		if e.Column != nil && e.Column.Name.L != col.Name.L {
			continue
		}

		desc := column.NewColDesc(col)

		// The FULL keyword causes the output to include the column collation and comments,
		// as well as the privileges you have for each column.
		row := &Row{}
		if e.Full {
			row.Data = types.MakeDatums(
				desc.Field,
				desc.Type,
				desc.Collation,
				desc.Null,
				desc.Key,
				desc.DefaultValue,
				desc.Extra,
				desc.Privileges,
				desc.Comment,
			)
		} else {
			row.Data = types.MakeDatums(
				desc.Field,
				desc.Type,
				desc.Null,
				desc.Key,
				desc.DefaultValue,
				desc.Extra,
			)
		}
		e.rows = append(e.rows, row)
	}
	return nil
}

func (e *ShowExec) fetchShowIndex() error {
	tb, err := e.getTable()
	if err != nil {
		return errors.Trace(err)
	}
	for _, idx := range tb.Indices() {
		for i, col := range idx.Columns {
			nonUniq := 1
			if idx.Unique {
				nonUniq = 0
			}
			var subPart interface{}
			if col.Length != types.UnspecifiedLength {
				subPart = col.Length
			}
			data := types.MakeDatums(
				tb.Meta().Name.O, // Table
				nonUniq,          // Non_unique
				idx.Name.O,       // Key_name
				i+1,              // Seq_in_index
				col.Name.O,       // Column_name
				"utf8_bin",       // Colation
				0,                // Cardinality
				subPart,          // Sub_part
				nil,              // Packed
				"YES",            // Null
				idx.Tp.String(),  // Index_type
				"",               // Comment
				idx.Comment,      // Index_comment
			)
			e.rows = append(e.rows, &Row{Data: data})
		}
	}
	return nil
}

func (e *ShowExec) fetchShowCharset() error {
	// See: http://dev.mysql.com/doc/refman/5.7/en/show-character-set.html
	descs := charset.GetAllCharsets()
	for _, desc := range descs {
		row := &Row{
			Data: types.MakeDatums(
				desc.Name,
				desc.Desc,
				desc.DefaultCollation,
				desc.Maxlen,
			),
		}
		e.rows = append(e.rows, row)
	}
	return nil
}

func (e *ShowExec) fetchShowVariables() error {
	sessionVars := variable.GetSessionVars(e.ctx)
	globalVars := variable.GetGlobalVarAccessor(e.ctx)
	for _, v := range variable.SysVars {
		var err error
		var value string
		if !e.GlobalScope {
			// Try to get Session Scope variable value first.
			sv, ok := sessionVars.Systems[v.Name]
			if ok {
				value = sv
			} else {
				// If session scope variable is not set, get the global scope value.
				value, err = globalVars.GetGlobalSysVar(e.ctx, v.Name)
				if err != nil {
					return errors.Trace(err)
				}
			}
		} else {
			value, err = globalVars.GetGlobalSysVar(e.ctx, v.Name)
			if err != nil {
				return errors.Trace(err)
			}
		}
		row := &Row{Data: types.MakeDatums(v.Name, value)}
		e.rows = append(e.rows, row)
	}
	return nil
}

func (e *ShowExec) fetchShowStatus() error {
	statusVars, err := variable.GetStatusVars()
	if err != nil {
		return errors.Trace(err)
	}
	for status, v := range statusVars {
		if e.GlobalScope && v.Scope == variable.ScopeSession {
			continue
		}
		value, err := types.ToString(v.Value)
		if err != nil {
			return errors.Trace(err)
		}
		row := &Row{Data: types.MakeDatums(status, value)}
		e.rows = append(e.rows, row)
	}
	return nil
}

func (e *ShowExec) fetchShowCreateTable() error {
	tb, err := e.getTable()
	if err != nil {
		return errors.Trace(err)
	}

	// TODO: let the result more like MySQL.
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("CREATE TABLE `%s` (\n", tb.Meta().Name.O))
	for i, col := range tb.Cols() {
		buf.WriteString(fmt.Sprintf("  `%s` %s", col.Name.O, col.GetTypeDesc()))
		if mysql.HasAutoIncrementFlag(col.Flag) {
			buf.WriteString(" NOT NULL AUTO_INCREMENT")
		} else {
			if mysql.HasNotNullFlag(col.Flag) {
				buf.WriteString(" NOT NULL")
			}
			switch col.DefaultValue {
			case nil:
				buf.WriteString(" DEFAULT NULL")
			case "CURRENT_TIMESTAMP":
				buf.WriteString(" DEFAULT CURRENT_TIMESTAMP")
			default:
				buf.WriteString(fmt.Sprintf(" DEFAULT '%v'", col.DefaultValue))
			}

			if mysql.HasOnUpdateNowFlag(col.Flag) {
				buf.WriteString(" ON UPDATE CURRENT_TIMESTAMP")
			}
		}
		if i != len(tb.Cols())-1 {
			buf.WriteString(",\n")
		}
	}

	if len(tb.Indices()) > 0 {
		buf.WriteString(",\n")
	}

	for i, idx := range tb.Indices() {
		if idx.Primary {
			buf.WriteString("  PRIMARY KEY ")
		} else if idx.Unique {
			buf.WriteString(fmt.Sprintf("  UNIQUE KEY `%s` ", idx.Name.O))
		} else {
			buf.WriteString(fmt.Sprintf("  KEY `%s` ", idx.Name.O))
		}

		cols := make([]string, 0, len(idx.Columns))
		for _, c := range idx.Columns {
			cols = append(cols, c.Name.O)
		}
		buf.WriteString(fmt.Sprintf("(`%s`)", strings.Join(cols, "`,`")))
		if i != len(tb.Indices())-1 {
			buf.WriteString(",\n")
		}
	}
	buf.WriteString("\n")

	buf.WriteString(") ENGINE=InnoDB")
	if s := tb.Meta().Charset; len(s) > 0 {
		buf.WriteString(fmt.Sprintf(" DEFAULT CHARSET=%s", s))
	} else {
		buf.WriteString(" DEFAULT CHARSET=latin1")
	}

	data := types.MakeDatums(tb.Meta().Name.O, buf.String())
	e.rows = append(e.rows, &Row{Data: data})
	return nil
}

func (e *ShowExec) fetchShowCollation() error {
	collations := charset.GetCollations()
	for _, v := range collations {
		isDefault := ""
		if v.IsDefault {
			isDefault = "Yes"
		}
		row := &Row{Data: types.MakeDatums(
			v.Name,
			v.CharsetName,
			v.ID,
			isDefault,
			"Yes",
			1,
		)}
		e.rows = append(e.rows, row)
	}
	return nil
}

func (e *ShowExec) fetchShowGrants() error {
	// Get checker
	checker := privilege.GetPrivilegeChecker(e.ctx)
	if checker == nil {
		return errors.New("Miss privilege checker!")
	}
	gs, err := checker.ShowGrants(e.ctx, e.User)
	if err != nil {
		return errors.Trace(err)
	}
	for _, g := range gs {
		data := types.MakeDatums(g)
		e.rows = append(e.rows, &Row{Data: data})
	}
	return nil
}

func (e *ShowExec) fetchShowTriggers() error {
	return nil
}

func (e *ShowExec) fetchShowProcedureStatus() error {
	return nil
}

func (e *ShowExec) getTable() (table.Table, error) {
	if e.Table == nil {
		return nil, errors.New("table not found")
	}
	tb, ok := e.is.TableByID(e.Table.TableInfo.ID)
	if !ok {
		return nil, errors.Errorf("table %s not found", e.Table.Name)
	}
	return tb, nil
}

// Close implements Executor Close interface.
func (e *ShowExec) Close() error {
	return nil
}
