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
	"fmt"
	"strings"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/column"
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/evaluator"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/sessionctx/variable"
	"github.com/pingcap/tidb/table"
	"github.com/pingcap/tidb/terror"
	"github.com/pingcap/tidb/util/types"
)

var (
	_ Executor = &UpdateExec{}
	_ Executor = &DeleteExec{}
	_ Executor = &InsertExec{}
)

// UpdateExec represents an update executor.
type UpdateExec struct {
	SelectExec  Executor
	OrderedList []*ast.Assignment

	// Map for unique (Table, handle) pair.
	updatedRowKeys map[table.Table]map[int64]struct{}
	ctx            context.Context

	rows        []*Row          // The rows fetched from TableExec.
	newRowsData [][]types.Datum // The new values to be set.
	fetched     bool
	cursor      int
}

// Next implements Executor Next interface.
func (e *UpdateExec) Next() (*Row, error) {
	if !e.fetched {
		err := e.fetchRows()
		if err != nil {
			return nil, errors.Trace(err)
		}
		e.fetched = true
	}

	columns, err := getUpdateColumns(e.OrderedList)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if e.cursor >= len(e.rows) {
		return nil, nil
	}
	if e.updatedRowKeys == nil {
		e.updatedRowKeys = make(map[table.Table]map[int64]struct{})
	}
	row := e.rows[e.cursor]
	newData := e.newRowsData[e.cursor]
	for _, entry := range row.RowKeys {
		tbl := entry.Tbl
		if e.updatedRowKeys[tbl] == nil {
			e.updatedRowKeys[tbl] = make(map[int64]struct{})
		}
		offset := e.getTableOffset(tbl)
		handle := entry.Handle
		oldData := row.Data[offset : offset+len(tbl.Cols())]
		newTableData := newData[offset : offset+len(tbl.Cols())]

		_, ok := e.updatedRowKeys[tbl][handle]
		if ok {
			// Each matching row is updated once, even if it matches the conditions multiple times.
			continue
		}
		// Update row
		err1 := updateRecord(e.ctx, handle, oldData, newTableData, columns, tbl, offset, false)
		if err1 != nil {
			return nil, errors.Trace(err1)
		}
		e.updatedRowKeys[tbl][handle] = struct{}{}
	}
	e.cursor++
	return &Row{}, nil
}

func getUpdateColumns(assignList []*ast.Assignment) (map[int]*ast.Assignment, error) {
	m := make(map[int]*ast.Assignment, len(assignList))
	for i, v := range assignList {
		m[i] = v
	}
	return m, nil
}

func (e *UpdateExec) fetchRows() error {
	for {
		row, err := e.SelectExec.Next()
		if err != nil {
			return errors.Trace(err)
		}
		if row == nil {
			return nil
		}
		data := make([]types.Datum, len(e.SelectExec.Fields()))
		newData := make([]types.Datum, len(e.SelectExec.Fields()))
		for i, f := range e.SelectExec.Fields() {
			data[i] = types.NewDatum(f.Expr.GetValue())
			newData[i] = data[i]
			if e.OrderedList[i] != nil {
				val, err := evaluator.Eval(e.ctx, e.OrderedList[i].Expr)
				if err != nil {
					return errors.Trace(err)
				}
				newData[i] = types.NewDatum(val)
			}
		}
		row.Data = data
		e.rows = append(e.rows, row)
		e.newRowsData = append(e.newRowsData, newData)
	}
}

func (e *UpdateExec) getTableOffset(t table.Table) int {
	fields := e.SelectExec.Fields()
	i := 0
	for i < len(fields) {
		field := fields[i]
		if field.Table.Name.L == t.Meta().Name.L {
			return i
		}
		i += len(field.Table.Columns)
	}
	return 0
}

func updateRecord(ctx context.Context, h int64, oldData, newData []types.Datum, updateColumns map[int]*ast.Assignment, t table.Table, offset int, onDuplicateUpdate bool) error {
	if err := t.LockRow(ctx, h, false); err != nil {
		return errors.Trace(err)
	}

	cols := t.Cols()
	touched := make(map[int]bool, len(cols))

	assignExists := false
	var newHandle types.Datum
	for i, asgn := range updateColumns {
		if asgn == nil {
			continue
		}
		if i < offset || i >= offset+len(cols) {
			// The assign expression is for another table, not this.
			continue
		}

		colIndex := i - offset
		col := cols[colIndex]
		if col.IsPKHandleColumn(t.Meta()) {
			newHandle = newData[i]
		}
		if mysql.HasAutoIncrementFlag(col.Flag) {
			if newData[i].Kind() == types.KindNull {
				return errors.Errorf("Column '%v' cannot be null", col.Name.O)
			}
			val, err := newData[i].ToInt64()
			if err != nil {
				return errors.Trace(err)
			}
			t.RebaseAutoID(val, true)
		}

		touched[colIndex] = true
		assignExists = true
	}

	// If no assign list for this table, no need to update.
	if !assignExists {
		return nil
	}

	// Check whether new value is valid.
	if err := column.CastValues(ctx, newData, cols); err != nil {
		return errors.Trace(err)
	}

	if err := column.CheckNotNull(cols, newData); err != nil {
		return errors.Trace(err)
	}

	// If row is not changed, we should do nothing.
	rowChanged := false
	for i := range oldData {
		if !touched[i] {
			continue
		}

		n, err := newData[i].CompareDatum(oldData[i])
		if err != nil {
			return errors.Trace(err)
		}
		if n != 0 {
			rowChanged = true
			break
		}
	}
	if !rowChanged {
		// See: https://dev.mysql.com/doc/refman/5.7/en/mysql-real-connect.html  CLIENT_FOUND_ROWS
		if variable.GetSessionVars(ctx).ClientCapability&mysql.ClientFoundRows > 0 {
			variable.GetSessionVars(ctx).AddAffectedRows(1)
		}
		return nil
	}

	var err error
	if newHandle.Kind() != types.KindNull {
		err = t.RemoveRecord(ctx, h, oldData)
		if err != nil {
			return errors.Trace(err)
		}
		_, err = t.AddRecord(ctx, newData)
	} else {
		// Update record to new value and update index.
		err = t.UpdateRecord(ctx, h, oldData, newData, touched)
	}
	if err != nil {
		return errors.Trace(err)
	}

	// Record affected rows.
	if !onDuplicateUpdate {
		variable.GetSessionVars(ctx).AddAffectedRows(1)
	} else {
		variable.GetSessionVars(ctx).AddAffectedRows(2)
	}
	return nil
}

// Fields implements Executor Fields interface.
// Returns nil to indicate there is no output.
func (e *UpdateExec) Fields() []*ast.ResultField {
	return nil
}

// Close implements Executor Close interface.
func (e *UpdateExec) Close() error {
	return e.SelectExec.Close()
}

// DeleteExec represents a delete executor.
// See: https://dev.mysql.com/doc/refman/5.7/en/delete.html
type DeleteExec struct {
	SelectExec Executor

	ctx          context.Context
	Tables       []*ast.TableName
	IsMultiTable bool

	finished bool
}

// Next implements Executor Next interface.
func (e *DeleteExec) Next() (*Row, error) {
	if e.finished {
		return nil, nil
	}
	defer func() {
		e.finished = true
	}()
	if e.IsMultiTable && len(e.Tables) == 0 {
		return &Row{}, nil
	}
	tblIDMap := make(map[int64]bool, len(e.Tables))
	// Get table alias map.
	tblNames := make(map[string]string)

	// Map for unique (Table, handle) pair.
	rowKeyMap := make(map[table.Table]map[int64]struct{})
	if e.IsMultiTable {
		// Delete from multiple tables should consider table ident list.
		fs := e.SelectExec.Fields()
		for _, f := range fs {
			if len(f.TableAsName.L) > 0 {
				tblNames[f.TableAsName.L] = f.TableName.Name.L
			} else {
				tblNames[f.TableName.Name.L] = f.TableName.Name.L
			}
		}
		for _, t := range e.Tables {
			// Consider DBName.
			_, ok := tblNames[t.Name.L]
			if !ok {
				return nil, errors.Errorf("Unknown table '%s' in MULTI DELETE", t.Name.O)
			}
			tblIDMap[t.TableInfo.ID] = true
		}
	}
	for {
		row, err := e.SelectExec.Next()
		if err != nil {
			return nil, errors.Trace(err)
		}
		if row == nil {
			break
		}

		for _, entry := range row.RowKeys {
			if e.IsMultiTable {
				tid := entry.Tbl.Meta().ID
				if _, ok := tblIDMap[tid]; !ok {
					continue
				}
			}
			if rowKeyMap[entry.Tbl] == nil {
				rowKeyMap[entry.Tbl] = make(map[int64]struct{})
			}
			rowKeyMap[entry.Tbl][entry.Handle] = struct{}{}
		}
	}
	for t, handleMap := range rowKeyMap {
		for handle := range handleMap {
			data, err := t.Row(e.ctx, handle)
			if err != nil {
				return nil, errors.Trace(err)
			}
			err = e.removeRow(e.ctx, t, handle, data)
			if err != nil {
				return nil, errors.Trace(err)
			}
		}
	}
	return nil, nil
}

func (e *DeleteExec) getTable(ctx context.Context, tableName *ast.TableName) (table.Table, error) {
	return sessionctx.GetDomain(ctx).InfoSchema().TableByName(tableName.Schema, tableName.Name)
}

func (e *DeleteExec) removeRow(ctx context.Context, t table.Table, h int64, data []types.Datum) error {
	err := t.RemoveRecord(ctx, h, data)
	if err != nil {
		return errors.Trace(err)
	}
	variable.GetSessionVars(ctx).AddAffectedRows(1)
	return nil
}

// Fields implements Executor Fields interface.
// Returns nil to indicate there is no output.
func (e *DeleteExec) Fields() []*ast.ResultField {
	return nil
}

// Close implements Executor Close interface.
func (e *DeleteExec) Close() error {
	return e.SelectExec.Close()
}

// InsertValues is the data to insert.
type InsertValues struct {
	currRow    int
	ctx        context.Context
	SelectExec Executor

	Table   table.Table
	Columns []*ast.ColumnName
	Lists   [][]ast.ExprNode
	Setlist []*ast.Assignment
}

// InsertExec represents an insert executor.
type InsertExec struct {
	*InsertValues

	OnDuplicate []*ast.Assignment
	fields      []*ast.ResultField

	Priority int

	finished bool
}

// Next implements Executor Next interface.
func (e *InsertExec) Next() (*Row, error) {
	if e.finished {
		return nil, nil
	}
	cols, err := e.getColumns(e.Table.Cols())
	if err != nil {
		return nil, errors.Trace(err)
	}
	txn, err := e.ctx.GetTxn(false)
	if err != nil {
		return nil, errors.Trace(err)
	}
	toUpdateColumns, err := getOnDuplicateUpdateColumns(e.OnDuplicate, e.Table)
	if err != nil {
		return nil, errors.Trace(err)
	}

	var rows [][]types.Datum
	if e.SelectExec != nil {
		rows, err = e.getRowsSelect(cols)
	} else {
		rows, err = e.getRows(cols)
	}
	if err != nil {
		return nil, errors.Trace(err)
	}

	for _, row := range rows {
		if len(e.OnDuplicate) == 0 {
			txn.SetOption(kv.PresumeKeyNotExists, nil)
		}
		h, err := e.Table.AddRecord(e.ctx, row)
		txn.DelOption(kv.PresumeKeyNotExists)
		if err == nil {
			continue
		}

		if len(e.OnDuplicate) == 0 || !terror.ErrorEqual(err, kv.ErrKeyExists) {
			return nil, errors.Trace(err)
		}
		if err = e.onDuplicateUpdate(row, h, toUpdateColumns); err != nil {
			return nil, errors.Trace(err)
		}
	}
	e.finished = true
	return nil, nil
}

// Fields implements Executor Fields interface.
// Returns nil to indicate there is no output.
func (e *InsertExec) Fields() []*ast.ResultField {
	return nil
}

// Close implements Executor Close interface.
func (e *InsertExec) Close() error {
	if e.SelectExec != nil {
		return e.SelectExec.Close()
	}
	return nil
}

// There are three types of insert statements:
// 1 insert ... values(...)  --> name type column
// 2 insert ... set x=y...   --> set type column
// 3 insert ... (select ..)  --> name type column
// See: https://dev.mysql.com/doc/refman/5.7/en/insert.html
func (e *InsertValues) getColumns(tableCols []*column.Col) ([]*column.Col, error) {
	var cols []*column.Col
	var err error

	if len(e.Setlist) > 0 {
		// Process `set` type column.
		columns := make([]string, 0, len(e.Setlist))
		for _, v := range e.Setlist {
			columns = append(columns, v.Column.Name.O)
		}

		cols, err = column.FindCols(tableCols, columns)
		if err != nil {
			return nil, errors.Errorf("INSERT INTO %s: %s", e.Table.Meta().Name.O, err)
		}

		if len(cols) == 0 {
			return nil, errors.Errorf("INSERT INTO %s: empty column", e.Table.Meta().Name.O)
		}
	} else {
		// Process `name` type column.
		columns := make([]string, 0, len(e.Columns))
		for _, v := range e.Columns {
			columns = append(columns, v.Name.O)
		}
		cols, err = column.FindCols(tableCols, columns)
		if err != nil {
			return nil, errors.Errorf("INSERT INTO %s: %s", e.Table.Meta().Name.O, err)
		}

		// If cols are empty, use all columns instead.
		if len(cols) == 0 {
			cols = tableCols
		}
	}

	// Check column whether is specified only once.
	err = column.CheckOnce(cols)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return cols, nil
}

func (e *InsertValues) fillValueList() error {
	if len(e.Setlist) > 0 {
		if len(e.Lists) > 0 {
			return errors.Errorf("INSERT INTO %s: set type should not use values", e.Table)
		}
		var l []ast.ExprNode
		for _, v := range e.Setlist {
			l = append(l, v.Expr)
		}
		e.Lists = append(e.Lists, l)
	}
	return nil
}

func (e *InsertValues) checkValueCount(insertValueCount, valueCount, num int, cols []*column.Col) error {
	if insertValueCount != valueCount {
		// "insert into t values (), ()" is valid.
		// "insert into t values (), (1)" is not valid.
		// "insert into t values (1), ()" is not valid.
		// "insert into t values (1,2), (1)" is not valid.
		// So the value count must be same for all insert list.
		return errors.Errorf("Column count doesn't match value count at row %d", num+1)
	}
	if valueCount == 0 && len(e.Columns) > 0 {
		// "insert into t (c1) values ()" is not valid.
		return errors.Errorf("INSERT INTO %s: expected %d value(s), have %d", e.Table.Meta().Name.O, len(e.Columns), 0)
	} else if valueCount > 0 && valueCount != len(cols) {
		return errors.Errorf("INSERT INTO %s: expected %d value(s), have %d", e.Table.Meta().Name.O, len(cols), valueCount)
	}
	return nil
}

func (e *InsertValues) getColumnDefaultValues(cols []*column.Col) (map[string]types.Datum, error) {
	defaultValMap := map[string]types.Datum{}
	for _, col := range cols {
		if value, ok, err := table.GetColDefaultValue(e.ctx, &col.ColumnInfo); ok {
			if err != nil {
				return nil, errors.Trace(err)
			}
			defaultValMap[col.Name.L] = value
		}
	}
	return defaultValMap, nil
}

func (e *InsertValues) getRows(cols []*column.Col) (rows [][]types.Datum, err error) {
	// process `insert|replace ... set x=y...`
	if err = e.fillValueList(); err != nil {
		return nil, errors.Trace(err)
	}

	defaultVals, err := e.getColumnDefaultValues(e.Table.Cols())
	if err != nil {
		return nil, errors.Trace(err)
	}

	rows = make([][]types.Datum, len(e.Lists))
	length := len(e.Lists[0])
	for i, list := range e.Lists {
		if err = e.checkValueCount(length, len(list), i, cols); err != nil {
			return nil, errors.Trace(err)
		}
		e.currRow = i
		rows[i], err = e.getRow(cols, list, defaultVals)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	return
}

func (e *InsertValues) getRow(cols []*column.Col, list []ast.ExprNode, defaultVals map[string]types.Datum) ([]types.Datum, error) {
	vals := make([]types.Datum, len(list))
	var err error
	for i, expr := range list {
		if d, ok := expr.(*ast.DefaultExpr); ok {
			cn := d.Name
			if cn != nil {
				var found bool
				vals[i], found = defaultVals[cn.Name.L]
				if !found {
					return nil, errors.Errorf("default column not found - %s", cn.Name.O)
				}
			} else {
				vals[i] = defaultVals[cols[i].Name.L]
			}
		} else {
			var val interface{}
			val, err = evaluator.Eval(e.ctx, expr)
			vals[i].SetValue(val)
			if err != nil {
				return nil, errors.Trace(err)
			}
		}
	}
	return e.fillRowData(cols, vals)
}

func (e *InsertValues) getRowsSelect(cols []*column.Col) ([][]types.Datum, error) {
	// process `insert|replace into ... select ... from ...`
	if len(e.SelectExec.Fields()) != len(cols) {
		return nil, errors.Errorf("Column count %d doesn't match value count %d", len(cols), len(e.SelectExec.Fields()))
	}
	var rows [][]types.Datum
	for {
		innerRow, err := e.SelectExec.Next()
		if err != nil {
			return nil, errors.Trace(err)
		}
		if innerRow == nil {
			break
		}
		e.currRow = len(rows)
		row, err := e.fillRowData(cols, innerRow.Data)
		if err != nil {
			return nil, errors.Trace(err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (e *InsertValues) fillRowData(cols []*column.Col, vals []types.Datum) ([]types.Datum, error) {
	row := make([]types.Datum, len(e.Table.Cols()))
	marked := make(map[int]struct{}, len(vals))
	for i, v := range vals {
		offset := cols[i].Offset
		row[offset] = v
		marked[offset] = struct{}{}
	}
	err := e.initDefaultValues(row, marked)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if err = column.CastValues(e.ctx, row, cols); err != nil {
		return nil, errors.Trace(err)
	}
	if err = column.CheckNotNull(e.Table.Cols(), row); err != nil {
		return nil, errors.Trace(err)
	}
	return row, nil
}

func (e *InsertValues) initDefaultValues(row []types.Datum, marked map[int]struct{}) error {
	var rewriteValueCol *column.Col
	var defaultValueCols []*column.Col
	for i, c := range e.Table.Cols() {
		if row[i].Kind() != types.KindNull {
			// Column value isn't nil and column isn't auto-increment, continue.
			if !mysql.HasAutoIncrementFlag(c.Flag) {
				continue
			}
			val, err := row[i].ToInt64()
			if err != nil {
				return errors.Trace(err)
			}
			if val != 0 {
				e.Table.RebaseAutoID(val, true)
				continue
			}
		}

		// If the nil value is evaluated in insert list, we will use nil except auto increment column.
		if _, ok := marked[i]; ok && !mysql.HasAutoIncrementFlag(c.Flag) && !mysql.HasTimestampFlag(c.Flag) {
			continue
		}

		if mysql.HasAutoIncrementFlag(c.Flag) {
			recordID, err := e.Table.AllocAutoID()
			if err != nil {
				return errors.Trace(err)
			}
			row[i].SetInt64(recordID)
			// Notes: incompatible with mysql
			// MySQL will set last insert id to the first row, as follows:
			// `t(id int AUTO_INCREMENT, c1 int, PRIMARY KEY (id))`
			// `insert t (c1) values(1),(2),(3);`
			// Last insert id will be 1, not 3.
			variable.GetSessionVars(e.ctx).SetLastInsertID(uint64(recordID))
			// It's used for retry.
			rewriteValueCol = c
		} else {
			var err error
			row[i], _, err = table.GetColDefaultValue(e.ctx, &c.ColumnInfo)
			if err != nil {
				return errors.Trace(err)
			}
		}

		defaultValueCols = append(defaultValueCols, c)
	}
	if err := column.CastValues(e.ctx, row, defaultValueCols); err != nil {
		return errors.Trace(err)
	}

	// It's used for retry.
	if rewriteValueCol == nil {
		return nil
	}
	if len(e.Setlist) > 0 {
		val := &ast.Assignment{
			Column: &ast.ColumnName{Name: rewriteValueCol.Name},
			Expr:   ast.NewValueExpr(row[rewriteValueCol.Offset].GetValue())}
		if len(e.Setlist) < rewriteValueCol.Offset+1 {
			e.Setlist = append(e.Setlist, val)
			return nil
		}
		setlist := make([]*ast.Assignment, 0, len(e.Setlist)+1)
		setlist = append(setlist, e.Setlist[:rewriteValueCol.Offset]...)
		setlist = append(setlist, val)
		e.Setlist = append(setlist, e.Setlist[rewriteValueCol.Offset:]...)
		return nil
	}

	// records the values of each row.
	vals := make([]ast.ExprNode, len(row))
	for i, col := range row {
		vals[i] = ast.NewValueExpr(col.GetValue())
	}
	if len(e.Lists) <= e.currRow {
		e.Lists = append(e.Lists, vals)
	} else {
		e.Lists[e.currRow] = vals
	}

	// records the column name only once.
	if e.currRow != len(e.Lists)-1 {
		return nil
	}
	if len(e.Columns) < rewriteValueCol.Offset+1 {
		e.Columns = append(e.Columns, &ast.ColumnName{Name: rewriteValueCol.Name})
		return nil
	}
	cols := make([]*ast.ColumnName, 0, len(e.Columns)+1)
	cols = append(cols, e.Columns[:rewriteValueCol.Offset]...)
	cols = append(cols, &ast.ColumnName{Name: rewriteValueCol.Name})
	e.Columns = append(cols, e.Columns[rewriteValueCol.Offset:]...)

	return nil
}

func (e *InsertExec) onDuplicateUpdate(row []types.Datum, h int64, cols map[int]*ast.Assignment) error {
	// On duplicate key update the duplicate row.
	// Evaluate the updated value.
	// TODO: report rows affected and last insert id.
	data, err := e.Table.Row(e.ctx, h)
	if err != nil {
		return errors.Trace(err)
	}
	// For evaluate ValuesExpr
	// http://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_values
	for i, rf := range e.fields {
		rf.Expr.SetValue(row[i].GetValue())
	}
	// Evaluate assignment
	newData := make([]types.Datum, len(data))
	for i, c := range row {
		asgn, ok := cols[i]
		if !ok {
			newData[i] = c
			continue
		}
		var val interface{}
		val, err = evaluator.Eval(e.ctx, asgn.Expr)
		if err != nil {
			return errors.Trace(err)
		}
		newData[i].SetValue(val)
	}
	if err = updateRecord(e.ctx, h, data, newData, cols, e.Table, 0, true); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func findColumnByName(t table.Table, name string) (*column.Col, error) {
	_, tableName, colName := splitQualifiedName(name)
	if len(tableName) > 0 && tableName != t.Meta().Name.O {
		return nil, errors.Errorf("unknown field %s.%s", tableName, colName)
	}

	c := column.FindCol(t.Cols(), colName)
	if c == nil {
		return nil, errors.Errorf("unknown field %s", colName)
	}
	return c, nil
}

func getOnDuplicateUpdateColumns(assignList []*ast.Assignment, t table.Table) (map[int]*ast.Assignment, error) {
	m := make(map[int]*ast.Assignment, len(assignList))

	for _, v := range assignList {
		col := v.Column
		c, err := findColumnByName(t, joinQualifiedName("", col.Table.L, col.Name.L))
		if err != nil {
			return nil, errors.Trace(err)
		}
		m[c.Offset] = v
	}
	return m, nil
}

// ReplaceExec represents a replace executor.
type ReplaceExec struct {
	*InsertValues
	Priority int
	finished bool
}

// Fields implements Executor Fields interface.
// Returns nil to indicate there is no output.
func (e *ReplaceExec) Fields() []*ast.ResultField {
	return nil
}

// Close implements Executor Close interface.
func (e *ReplaceExec) Close() error {
	if e.SelectExec != nil {
		return e.SelectExec.Close()
	}
	return nil
}

// Next implements Executor Next interface.
func (e *ReplaceExec) Next() (*Row, error) {
	if e.finished {
		return nil, nil
	}
	cols, err := e.getColumns(e.Table.Cols())
	if err != nil {
		return nil, errors.Trace(err)
	}

	var rows [][]types.Datum
	if e.SelectExec != nil {
		rows, err = e.getRowsSelect(cols)
	} else {
		rows, err = e.getRows(cols)
	}
	if err != nil {
		return nil, errors.Trace(err)
	}

	for _, row := range rows {
		h, err := e.Table.AddRecord(e.ctx, row)
		if err == nil {
			continue
		}
		if err != nil && !terror.ErrorEqual(err, kv.ErrKeyExists) {
			return nil, errors.Trace(err)
		}

		// While the insertion fails because a duplicate-key error occurs for a primary key or unique index,
		// a storage engine may perform the REPLACE as an update rather than a delete plus insert.
		// See: http://dev.mysql.com/doc/refman/5.7/en/replace.html.
		if err = e.replaceRow(h, row); err != nil {
			return nil, errors.Trace(err)
		}
		variable.GetSessionVars(e.ctx).AddAffectedRows(1)
	}
	e.finished = true
	return nil, nil
}

func (e *ReplaceExec) replaceRow(handle int64, replaceRow []types.Datum) error {
	row, err := e.Table.Row(e.ctx, handle)
	if err != nil {
		return errors.Trace(err)
	}
	isReplace := false
	touched := make(map[int]bool, len(row))
	for i, val := range row {
		v, err1 := val.CompareDatum(replaceRow[i])
		if err1 != nil {
			return errors.Trace(err1)
		}
		if v != 0 {
			touched[i] = true
			isReplace = true
		}
	}
	if isReplace {
		variable.GetSessionVars(e.ctx).AddAffectedRows(1)
		if err = e.Table.UpdateRecord(e.ctx, handle, row, replaceRow, touched); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// SplitQualifiedName splits an identifier name to db, table and field name.
func splitQualifiedName(name string) (db string, table string, field string) {
	seps := strings.Split(name, ".")

	l := len(seps)
	switch l {
	case 1:
		// `name` is field.
		field = seps[0]
	case 2:
		// `name` is `table.field`.
		table, field = seps[0], seps[1]
	case 3:
		// `name` is `db.table.field`.
		db, table, field = seps[0], seps[1], seps[2]
	default:
		// `name` is `db.table.field`.
		db, table, field = seps[l-3], seps[l-2], seps[l-1]
	}

	return
}

// JoinQualifiedName converts db, table, field to a qualified name.
func joinQualifiedName(db string, table string, field string) string {
	if len(db) > 0 {
		return fmt.Sprintf("%s.%s.%s", db, table, field)
	} else if len(table) > 0 {
		return fmt.Sprintf("%s.%s", table, field)
	} else {
		return field
	}
}
