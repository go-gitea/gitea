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

package executor

import (
	"sort"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/column"
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/evaluator"
	"github.com/pingcap/tidb/inspectkv"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/optimizer/plan"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/sessionctx/db"
	"github.com/pingcap/tidb/sessionctx/forupdate"
	"github.com/pingcap/tidb/table"
	"github.com/pingcap/tidb/terror"
	"github.com/pingcap/tidb/util/codec"
	"github.com/pingcap/tidb/util/distinct"
	"github.com/pingcap/tidb/util/types"
)

var (
	_ Executor = &AggregateExec{}
	_ Executor = &CheckTableExec{}
	_ Executor = &FilterExec{}
	_ Executor = &IndexRangeExec{}
	_ Executor = &IndexScanExec{}
	_ Executor = &LimitExec{}
	_ Executor = &SelectFieldsExec{}
	_ Executor = &SelectLockExec{}
	_ Executor = &ShowDDLExec{}
	_ Executor = &SortExec{}
	_ Executor = &TableScanExec{}
)

// Error instances.
var (
	ErrUnknownPlan     = terror.ClassExecutor.New(CodeUnknownPlan, "Unknown plan")
	ErrPrepareMulti    = terror.ClassExecutor.New(CodePrepareMulti, "Can not prepare multiple statements")
	ErrStmtNotFound    = terror.ClassExecutor.New(CodeStmtNotFound, "Prepared statement not found")
	ErrSchemaChanged   = terror.ClassExecutor.New(CodeSchemaChanged, "Schema has changed")
	ErrWrongParamCount = terror.ClassExecutor.New(CodeWrongParamCount, "Wrong parameter count")
)

// Error codes.
const (
	CodeUnknownPlan     terror.ErrCode = 1
	CodePrepareMulti    terror.ErrCode = 2
	CodeStmtNotFound    terror.ErrCode = 3
	CodeSchemaChanged   terror.ErrCode = 4
	CodeWrongParamCount terror.ErrCode = 5
)

// Row represents a record row.
type Row struct {
	// Data is the output record data for current Plan.
	Data []types.Datum

	RowKeys []*RowKeyEntry
}

// RowKeyEntry is designed for Delete statement in multi-table mode,
// we should know which table this row comes from.
type RowKeyEntry struct {
	// The table which this row come from.
	Tbl table.Table
	// Row key.
	Handle int64
}

// Executor executes a query.
type Executor interface {
	Fields() []*ast.ResultField
	Next() (*Row, error)
	Close() error
}

// ShowDDLExec represents a show DDL executor.
type ShowDDLExec struct {
	fields []*ast.ResultField
	ctx    context.Context
	done   bool
}

// Fields implements Executor Fields interface.
func (e *ShowDDLExec) Fields() []*ast.ResultField {
	return e.fields
}

// Next implements Execution Next interface.
func (e *ShowDDLExec) Next() (*Row, error) {
	if e.done {
		return nil, nil
	}

	txn, err := e.ctx.GetTxn(false)
	if err != nil {
		return nil, errors.Trace(err)
	}

	ddlInfo, err := inspectkv.GetDDLInfo(txn)
	if err != nil {
		return nil, errors.Trace(err)
	}
	bgInfo, err := inspectkv.GetBgDDLInfo(txn)
	if err != nil {
		return nil, errors.Trace(err)
	}

	var ddlOwner, ddlJob string
	if ddlInfo.Owner != nil {
		ddlOwner = ddlInfo.Owner.String()
	}
	if ddlInfo.Job != nil {
		ddlJob = ddlInfo.Job.String()
	}

	var bgOwner, bgJob string
	if bgInfo.Owner != nil {
		bgOwner = bgInfo.Owner.String()
	}
	if bgInfo.Job != nil {
		bgJob = bgInfo.Job.String()
	}

	row := &Row{}
	row.Data = types.MakeDatums(
		ddlInfo.SchemaVer,
		ddlOwner,
		ddlJob,
		bgInfo.SchemaVer,
		bgOwner,
		bgJob,
	)
	for i, f := range e.fields {
		f.Expr.SetValue(row.Data[i].GetValue())
	}
	e.done = true

	return row, nil
}

// Close implements Executor Close interface.
func (e *ShowDDLExec) Close() error {
	return nil
}

// CheckTableExec represents a check table executor.
type CheckTableExec struct {
	tables []*ast.TableName
	ctx    context.Context
	done   bool
}

// Fields implements Executor Fields interface.
func (e *CheckTableExec) Fields() []*ast.ResultField {
	return nil
}

// Next implements Execution Next interface.
func (e *CheckTableExec) Next() (*Row, error) {
	if e.done {
		return nil, nil
	}

	dbName := model.NewCIStr(db.GetCurrentSchema(e.ctx))
	is := sessionctx.GetDomain(e.ctx).InfoSchema()

	for _, t := range e.tables {
		tb, err := is.TableByName(dbName, t.Name)
		if err != nil {
			return nil, errors.Trace(err)
		}
		for _, idx := range tb.Indices() {
			txn, err := e.ctx.GetTxn(false)
			if err != nil {
				return nil, errors.Trace(err)
			}
			err = inspectkv.CompareIndexData(txn, tb, idx)
			if err != nil {
				return nil, errors.Errorf("%v err:%v", t.Name, err)
			}
		}
	}
	e.done = true

	return nil, nil
}

// Close implements plan.Plan Close interface.
func (e *CheckTableExec) Close() error {
	return nil
}

// TableScanExec represents a table scan executor.
type TableScanExec struct {
	t          table.Table
	fields     []*ast.ResultField
	iter       kv.Iterator
	ctx        context.Context
	ranges     []plan.TableRange // Disjoint close handle ranges.
	seekHandle int64             // The handle to seek, should be initialized to math.MinInt64.
	cursor     int               // The range cursor, used to locate to current range.
}

// Fields implements Executor Fields interface.
func (e *TableScanExec) Fields() []*ast.ResultField {
	return e.fields
}

// Next implements Execution Next interface.
func (e *TableScanExec) Next() (*Row, error) {
	for {
		if e.cursor >= len(e.ranges) {
			return nil, nil
		}
		ran := e.ranges[e.cursor]
		if e.seekHandle < ran.LowVal {
			e.seekHandle = ran.LowVal
		}
		if e.seekHandle > ran.HighVal {
			e.cursor++
			continue
		}
		handle, found, err := e.t.Seek(e.ctx, e.seekHandle)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if !found {
			return nil, nil
		}
		if handle > ran.HighVal {
			// The handle is out of the current range, but may be in following ranges.
			// We seek to the range that may contains the handle, so we
			// don't need to seek key again.
			inRange := e.seekRange(handle)
			if !inRange {
				// The handle may be less than the current range low value, can not
				// return directly.
				continue
			}
		}
		row, err := e.getRow(handle)
		if err != nil {
			return nil, errors.Trace(err)
		}
		e.seekHandle = handle + 1
		return row, nil
	}
}

// seekRange increments the range cursor to the range
// with high value greater or equal to handle.
func (e *TableScanExec) seekRange(handle int64) (inRange bool) {
	for {
		e.cursor++
		if e.cursor >= len(e.ranges) {
			return false
		}
		ran := e.ranges[e.cursor]
		if handle < ran.LowVal {
			return false
		}
		if handle > ran.HighVal {
			continue
		}
		return true
	}
}

func (e *TableScanExec) getRow(handle int64) (*Row, error) {
	row := &Row{}
	var err error
	row.Data, err = e.t.Row(e.ctx, handle)
	if err != nil {
		return nil, errors.Trace(err)
	}
	// Set result fields value.
	for i, v := range e.fields {
		v.Expr.SetValue(row.Data[i].GetValue())
	}

	// Put rowKey to the tail of record row
	rke := &RowKeyEntry{
		Tbl:    e.t,
		Handle: handle,
	}
	row.RowKeys = append(row.RowKeys, rke)
	return row, nil
}

// Close implements Executor Close interface.
func (e *TableScanExec) Close() error {
	if e.iter != nil {
		e.iter.Close()
		e.iter = nil
	}
	return nil
}

// IndexRangeExec represents an index range scan executor.
type IndexRangeExec struct {
	scan *IndexScanExec

	// seekVal is different from lowVal, it is casted from lowVal and
	// must be less than or equal to lowVal, used to seek the index.
	lowVals     []types.Datum
	lowExclude  bool
	highVals    []types.Datum
	highExclude bool

	iter       kv.IndexIterator
	skipLowCmp bool
	finished   bool
}

// Fields implements Executor Fields interface.
func (e *IndexRangeExec) Fields() []*ast.ResultField {
	return e.scan.fields
}

// Next implements Executor Next interface.
func (e *IndexRangeExec) Next() (*Row, error) {
	if e.iter == nil {
		seekVals := make([]types.Datum, len(e.scan.idx.Columns))
		for i := 0; i < len(e.lowVals); i++ {
			if e.lowVals[i].Kind() == types.KindMinNotNull {
				seekVals[i].SetBytes([]byte{})
			} else {
				val, err := e.lowVals[i].ConvertTo(e.scan.valueTypes[i])
				seekVals[i] = val
				if err != nil {
					return nil, errors.Trace(err)
				}
			}
		}
		txn, err := e.scan.ctx.GetTxn(false)
		if err != nil {
			return nil, errors.Trace(err)
		}
		e.iter, _, err = e.scan.idx.X.Seek(txn, seekVals)
		if err != nil {
			return nil, types.EOFAsNil(err)
		}
	}

	for {
		if e.finished {
			return nil, nil
		}
		idxKey, h, err := e.iter.Next()
		if err != nil {
			return nil, types.EOFAsNil(err)
		}
		if !e.skipLowCmp {
			var cmp int
			cmp, err = indexCompare(idxKey, e.lowVals)
			if err != nil {
				return nil, errors.Trace(err)
			}
			if cmp < 0 || (cmp == 0 && e.lowExclude) {
				continue
			}
			e.skipLowCmp = true
		}
		cmp, err := indexCompare(idxKey, e.highVals)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if cmp > 0 || (cmp == 0 && e.highExclude) {
			// This span has finished iteration.
			e.finished = true
			continue
		}
		var row *Row
		row, err = e.lookupRow(h)
		if err != nil {
			return nil, errors.Trace(err)
		}
		return row, nil
	}
}

// indexCompare compares multi column index.
// The length of boundVals may be less than idxKey.
func indexCompare(idxKey []types.Datum, boundVals []types.Datum) (int, error) {
	for i := 0; i < len(boundVals); i++ {
		cmp, err := idxKey[i].CompareDatum(boundVals[i])
		if err != nil {
			return -1, errors.Trace(err)
		}
		if cmp != 0 {
			return cmp, nil
		}
	}
	return 0, nil
}

func (e *IndexRangeExec) lookupRow(h int64) (*Row, error) {
	row := &Row{}
	var err error
	row.Data, err = e.scan.tbl.Row(e.scan.ctx, h)
	if err != nil {
		return nil, errors.Trace(err)
	}
	rowKey := &RowKeyEntry{
		Tbl:    e.scan.tbl,
		Handle: h,
	}
	row.RowKeys = append(row.RowKeys, rowKey)
	return row, nil
}

// Close implements Executor Close interface.
func (e *IndexRangeExec) Close() error {
	if e.iter != nil {
		e.iter.Close()
		e.iter = nil
	}
	e.finished = false
	e.skipLowCmp = false
	return nil
}

// IndexScanExec represents an index scan executor.
type IndexScanExec struct {
	tbl        table.Table
	idx        *column.IndexedCol
	fields     []*ast.ResultField
	Ranges     []*IndexRangeExec
	Desc       bool
	rangeIdx   int
	ctx        context.Context
	valueTypes []*types.FieldType
}

// Fields implements Executor Fields interface.
func (e *IndexScanExec) Fields() []*ast.ResultField {
	return e.fields
}

// Next implements Executor Next interface.
func (e *IndexScanExec) Next() (*Row, error) {
	for e.rangeIdx < len(e.Ranges) {
		ran := e.Ranges[e.rangeIdx]
		row, err := ran.Next()
		if err != nil {
			return nil, errors.Trace(err)
		}
		if row != nil {
			for i, val := range row.Data {
				e.fields[i].Expr.SetValue(val.GetValue())
			}
			return row, nil
		}
		ran.Close()
		e.rangeIdx++
	}
	return nil, nil
}

// Close implements Executor Close interface.
func (e *IndexScanExec) Close() error {
	for e.rangeIdx < len(e.Ranges) {
		e.Ranges[e.rangeIdx].Close()
		e.rangeIdx++
	}
	return nil
}

// JoinOuterExec represents an outer join executor.
type JoinOuterExec struct {
	OuterExec Executor
	InnerPlan plan.Plan
	innerExec Executor
	fields    []*ast.ResultField
	builder   *executorBuilder
	gotRow    bool
}

// Fields implements Executor Fields interface.
func (e *JoinOuterExec) Fields() []*ast.ResultField {
	return e.fields
}

// Next implements Executor Next interface.
// The data in the returned row is not used by caller.
// If inner executor didn't get any row for an outer executor row,
// a row with 0 len Data indicates there is no inner row matched for
// an outer row.
func (e *JoinOuterExec) Next() (*Row, error) {
	var rowKeys []*RowKeyEntry
	for {
		if e.innerExec == nil {
			e.gotRow = false
			outerRow, err := e.OuterExec.Next()
			if err != nil {
				return nil, errors.Trace(err)
			}
			if outerRow == nil {
				return nil, nil
			}
			rowKeys = outerRow.RowKeys
			plan.Refine(e.InnerPlan)
			e.innerExec = e.builder.build(e.InnerPlan)
			if e.builder.err != nil {
				return nil, errors.Trace(e.builder.err)
			}
		}
		row, err := e.innerExec.Next()
		if err != nil {
			return nil, errors.Trace(err)
		}
		if row == nil {
			e.innerExec.Close()
			e.innerExec = nil
			if e.gotRow {
				continue
			}
			e.setInnerNull()
			return &Row{RowKeys: rowKeys}, nil
		}
		if len(row.Data) != 0 {
			e.gotRow = true
			row.RowKeys = append(rowKeys, row.RowKeys...)
			return row, nil
		}
	}
}

func (e *JoinOuterExec) setInnerNull() {
	for _, rf := range e.InnerPlan.Fields() {
		rf.Expr.SetValue(nil)
	}
}

// Close implements Executor Close interface.
func (e *JoinOuterExec) Close() error {
	err := e.OuterExec.Close()
	if e.innerExec != nil {
		return errors.Trace(e.innerExec.Close())
	}
	return errors.Trace(err)
}

// JoinInnerExec represents an inner join executor.
type JoinInnerExec struct {
	InnerPlans []plan.Plan
	innerExecs []Executor
	Condition  ast.ExprNode
	ctx        context.Context
	fields     []*ast.ResultField
	builder    *executorBuilder
	done       bool
	cursor     int
}

// Fields implements Executor Fields interface.
func (e *JoinInnerExec) Fields() []*ast.ResultField {
	return e.fields
}

// Next implements Executor Next interface.
// The data in the returned row is not used by caller.
func (e *JoinInnerExec) Next() (*Row, error) {
	if e.done {
		return nil, nil
	}
	rowKeysSlice := make([][]*RowKeyEntry, len(e.InnerPlans))
	for {
		exec := e.innerExecs[e.cursor]
		if exec == nil {
			innerPlan := e.InnerPlans[e.cursor]
			plan.Refine(innerPlan)
			exec = e.builder.build(innerPlan)
			if e.builder.err != nil {
				return nil, errors.Trace(e.builder.err)
			}
			e.innerExecs[e.cursor] = exec
		}
		row, err := exec.Next()
		if err != nil {
			return nil, errors.Trace(err)
		}
		if row == nil {
			exec.Close()
			e.innerExecs[e.cursor] = nil
			if e.cursor == 0 {
				e.done = true
				return nil, nil
			}
			e.cursor--
			continue
		}
		rowKeysSlice[e.cursor] = row.RowKeys
		if e.cursor < len(e.innerExecs)-1 {
			e.cursor++
			continue
		}
		var match = true
		if e.Condition != nil {
			match, err = evaluator.EvalBool(e.ctx, e.Condition)
			if err != nil {
				return nil, errors.Trace(err)
			}
		}
		if match {
			row.RowKeys = joinRowKeys(rowKeysSlice)
			return row, nil
		}
	}
}

func joinRowKeys(rowKeysSlice [][]*RowKeyEntry) []*RowKeyEntry {
	count := 0
	for _, rowKeys := range rowKeysSlice {
		count += len(rowKeys)
	}
	joined := make([]*RowKeyEntry, count)
	offset := 0
	for _, rowKeys := range rowKeysSlice {
		copy(joined[offset:], rowKeys)
		offset += len(rowKeys)
	}
	return joined
}

// Close implements Executor Close interface.
func (e *JoinInnerExec) Close() error {
	var err error
	for _, inExec := range e.innerExecs {
		if inExec != nil {
			e := inExec.Close()
			if e != nil {
				err = errors.Trace(e)
			}
		}
	}
	return err
}

// SelectFieldsExec represents a select fields executor.
type SelectFieldsExec struct {
	Src          Executor
	ResultFields []*ast.ResultField
	executed     bool
	ctx          context.Context
}

// Fields implements Executor Fields interface.
func (e *SelectFieldsExec) Fields() []*ast.ResultField {
	return e.ResultFields
}

// Next implements Executor Next interface.
func (e *SelectFieldsExec) Next() (*Row, error) {
	var rowKeys []*RowKeyEntry
	if e.Src != nil {
		srcRow, err := e.Src.Next()
		if err != nil {
			return nil, errors.Trace(err)
		}
		if srcRow == nil {
			return nil, nil
		}
		rowKeys = srcRow.RowKeys
	} else {
		// If Src is nil, only one row should be returned.
		if e.executed {
			return nil, nil
		}
	}
	e.executed = true
	row := &Row{
		RowKeys: rowKeys,
		Data:    make([]types.Datum, len(e.ResultFields)),
	}
	for i, field := range e.ResultFields {
		val, err := evaluator.Eval(e.ctx, field.Expr)
		if err != nil {
			return nil, errors.Trace(err)
		}
		row.Data[i] = types.NewDatum(val)
	}
	return row, nil
}

// Close implements Executor Close interface.
func (e *SelectFieldsExec) Close() error {
	if e.Src != nil {
		return e.Src.Close()
	}
	return nil
}

// FilterExec represents a filter executor.
type FilterExec struct {
	Src       Executor
	Condition ast.ExprNode
	ctx       context.Context
}

// Fields implements Executor Fields interface.
func (e *FilterExec) Fields() []*ast.ResultField {
	return e.Src.Fields()
}

// Next implements Executor Next interface.
func (e *FilterExec) Next() (*Row, error) {
	for {
		srcRow, err := e.Src.Next()
		if err != nil {
			return nil, errors.Trace(err)
		}
		if srcRow == nil {
			return nil, nil
		}
		match, err := evaluator.EvalBool(e.ctx, e.Condition)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if match {
			return srcRow, nil
		}
	}
}

// Close implements Executor Close interface.
func (e *FilterExec) Close() error {
	return e.Src.Close()
}

// SelectLockExec represents a select lock executor.
type SelectLockExec struct {
	Src  Executor
	Lock ast.SelectLockType
	ctx  context.Context
}

// Fields implements Executor Fields interface.
func (e *SelectLockExec) Fields() []*ast.ResultField {
	return e.Src.Fields()
}

// Next implements Executor Next interface.
func (e *SelectLockExec) Next() (*Row, error) {
	row, err := e.Src.Next()
	if err != nil {
		return nil, errors.Trace(err)
	}
	if row == nil {
		return nil, nil
	}
	if len(row.RowKeys) != 0 && e.Lock == ast.SelectLockForUpdate {
		forupdate.SetForUpdate(e.ctx)
		for _, k := range row.RowKeys {
			err = k.Tbl.LockRow(e.ctx, k.Handle, true)
			if err != nil {
				return nil, errors.Trace(err)
			}
		}
	}
	return row, nil
}

// Close implements Executor Close interface.
func (e *SelectLockExec) Close() error {
	return e.Src.Close()
}

// LimitExec represents limit executor
type LimitExec struct {
	Src    Executor
	Offset uint64
	Count  uint64
	Idx    uint64
}

// Fields implements Executor Fields interface.
func (e *LimitExec) Fields() []*ast.ResultField {
	return e.Src.Fields()
}

// Next implements Executor Next interface.
func (e *LimitExec) Next() (*Row, error) {
	for e.Idx < e.Offset {
		srcRow, err := e.Src.Next()
		if err != nil {
			return nil, errors.Trace(err)
		}
		if srcRow == nil {
			return nil, nil
		}
		e.Idx++
	}
	// Negative Limit means no limit.
	if e.Count >= 0 && e.Idx >= e.Offset+e.Count {
		return nil, nil
	}
	srcRow, err := e.Src.Next()
	if err != nil {
		return nil, errors.Trace(err)
	}
	if srcRow == nil {
		return nil, nil
	}
	e.Idx++
	return srcRow, nil
}

// Close implements Executor Close interface.
func (e *LimitExec) Close() error {
	return e.Src.Close()
}

// orderByRow binds a row to its order values, so it can be sorted.
type orderByRow struct {
	key []interface{}
	row *Row
}

// SortExec represents sorting executor.
type SortExec struct {
	Src     Executor
	ByItems []*ast.ByItem
	Rows    []*orderByRow
	ctx     context.Context
	Idx     int
	fetched bool
	err     error
}

// Fields implements Executor Fields interface.
func (e *SortExec) Fields() []*ast.ResultField {
	return e.Src.Fields()
}

// Len returns the number of rows.
func (e *SortExec) Len() int {
	return len(e.Rows)
}

// Swap implements sort.Interface Swap interface.
func (e *SortExec) Swap(i, j int) {
	e.Rows[i], e.Rows[j] = e.Rows[j], e.Rows[i]
}

// Less implements sort.Interface Less interface.
func (e *SortExec) Less(i, j int) bool {
	for index, by := range e.ByItems {
		v1 := e.Rows[i].key[index]
		v2 := e.Rows[j].key[index]

		ret, err := types.Compare(v1, v2)
		if err != nil {
			e.err = err
			return true
		}

		if by.Desc {
			ret = -ret
		}

		if ret < 0 {
			return true
		} else if ret > 0 {
			return false
		}
	}

	return false
}

// Next implements Executor Next interface.
func (e *SortExec) Next() (*Row, error) {
	if !e.fetched {
		for {
			srcRow, err := e.Src.Next()
			if err != nil {
				return nil, errors.Trace(err)
			}
			if srcRow == nil {
				break
			}
			orderRow := &orderByRow{
				row: srcRow,
				key: make([]interface{}, len(e.ByItems)),
			}
			for i, byItem := range e.ByItems {
				orderRow.key[i], err = evaluator.Eval(e.ctx, byItem.Expr)
				if err != nil {
					return nil, errors.Trace(err)
				}
			}
			e.Rows = append(e.Rows, orderRow)
		}
		sort.Sort(e)
		e.fetched = true
	}
	if e.err != nil {
		return nil, errors.Trace(e.err)
	}
	if e.Idx >= len(e.Rows) {
		return nil, nil
	}
	row := e.Rows[e.Idx].row
	e.Idx++
	return row, nil
}

// Close implements Executor Close interface.
func (e *SortExec) Close() error {
	return e.Src.Close()
}

// For select stmt with aggregate function but without groupby clasue,
// We consider there is a single group with key singleGroup.
const singleGroup = "SingleGroup"

// AggregateExec deals with all the aggregate functions.
// It is built from Aggregate Plan. When Next() is called, it reads all the data from Src and updates all the items in AggFuncs.
// TODO: Support having.
type AggregateExec struct {
	Src               Executor
	ResultFields      []*ast.ResultField
	executed          bool
	ctx               context.Context
	finish            bool
	AggFuncs          []*ast.AggregateFuncExpr
	groupMap          map[string]bool
	groups            []string
	currentGroupIndex int
	GroupByItems      []*ast.ByItem
}

// Fields implements Executor Fields interface.
func (e *AggregateExec) Fields() []*ast.ResultField {
	return e.ResultFields
}

// Next implements Executor Next interface.
func (e *AggregateExec) Next() (*Row, error) {
	// In this stage we consider all data from src as a single group.
	if !e.executed {
		e.groupMap = make(map[string]bool)
		e.groups = []string{}
		for {
			hasMore, err := e.innerNext()
			if err != nil {
				return nil, errors.Trace(err)
			}
			if !hasMore {
				break
			}
		}
		e.executed = true
		if (len(e.groups) == 0) && (len(e.GroupByItems) == 0) {
			// If no groupby and no data, we should add an empty group.
			// For example:
			// "select count(c) from t;" should return one row [0]
			// "select count(c) from t group by c1;" should return empty result set.
			e.groups = append(e.groups, singleGroup)
		}
	}
	if e.currentGroupIndex >= len(e.groups) {
		return nil, nil
	}
	groupKey := e.groups[e.currentGroupIndex]
	for _, af := range e.AggFuncs {
		af.CurrentGroup = groupKey
	}
	e.currentGroupIndex++
	return &Row{}, nil
}

func (e *AggregateExec) getGroupKey() (string, error) {
	if len(e.GroupByItems) == 0 {
		return singleGroup, nil
	}
	vals := make([]types.Datum, 0, len(e.GroupByItems))
	for _, item := range e.GroupByItems {
		v, err := evaluator.Eval(e.ctx, item.Expr)
		if err != nil {
			return "", errors.Trace(err)
		}
		vals = append(vals, types.NewDatum(v))
	}
	bs, err := codec.EncodeValue([]byte{}, vals...)
	if err != nil {
		return "", errors.Trace(err)
	}
	return string(bs), nil
}

// Fetch a single row from src and update each aggregate function.
// If the first return value is false, it means there is no more data from src.
func (e *AggregateExec) innerNext() (bool, error) {
	if e.Src != nil {
		srcRow, err := e.Src.Next()
		if err != nil {
			return false, errors.Trace(err)
		}
		if srcRow == nil {
			return false, nil
		}
	} else {
		// If Src is nil, only one row should be returned.
		if e.executed {
			return false, nil
		}
	}
	e.executed = true
	groupKey, err := e.getGroupKey()
	if err != nil {
		return false, errors.Trace(err)
	}
	if _, ok := e.groupMap[groupKey]; !ok {
		e.groupMap[groupKey] = true
		e.groups = append(e.groups, groupKey)
	}
	for _, af := range e.AggFuncs {
		for _, arg := range af.Args {
			_, err := evaluator.Eval(e.ctx, arg)
			if err != nil {
				return false, errors.Trace(err)
			}
		}
		af.CurrentGroup = groupKey
		af.Update()
	}
	return true, nil
}

// Close implements Executor Close interface.
func (e *AggregateExec) Close() error {
	for _, af := range e.AggFuncs {
		af.Clear()
	}
	if e.Src != nil {
		return e.Src.Close()
	}
	return nil
}

// UnionExec represents union executor.
type UnionExec struct {
	fields []*ast.ResultField
	Sels   []Executor
	cursor int
}

// Fields implements Executor Fields interface.
func (e *UnionExec) Fields() []*ast.ResultField {
	return e.fields
}

// Next implements Executor Next interface.
func (e *UnionExec) Next() (*Row, error) {
	for {
		if e.cursor >= len(e.Sels) {
			return nil, nil
		}
		sel := e.Sels[e.cursor]
		row, err := sel.Next()
		if err != nil {
			return nil, errors.Trace(err)
		}
		if row == nil {
			e.cursor++
			continue
		}
		if e.cursor != 0 {
			for i := range row.Data {
				// The column value should be casted as the same type of the first select statement in corresponding position
				rf := e.fields[i]
				var val types.Datum
				val, err = row.Data[i].ConvertTo(&rf.Column.FieldType)
				if err != nil {
					return nil, errors.Trace(err)
				}
				row.Data[i] = val
			}
		}
		for i, v := range row.Data {
			e.fields[i].Expr.SetValue(v.GetValue())
		}
		return row, nil
	}
}

// Close implements Executor Close interface.
func (e *UnionExec) Close() error {
	var err error
	for _, sel := range e.Sels {
		er := sel.Close()
		if er != nil {
			err = errors.Trace(er)
		}
	}
	return err
}

// DistinctExec represents Distinct executor.
type DistinctExec struct {
	Src     Executor
	checker *distinct.Checker
}

// Fields implements Executor Fields interface.
func (e *DistinctExec) Fields() []*ast.ResultField {
	return e.Src.Fields()
}

// Next implements Executor Next interface.
func (e *DistinctExec) Next() (*Row, error) {
	if e.checker == nil {
		e.checker = distinct.CreateDistinctChecker()
	}
	for {
		row, err := e.Src.Next()
		if err != nil {
			return nil, errors.Trace(err)
		}
		if row == nil {
			return nil, nil
		}
		ok, err := e.checker.Check(types.DatumsToInterfaces(row.Data))
		if err != nil {
			return nil, errors.Trace(err)
		}
		if !ok {
			continue
		}
		return row, nil
	}
}

// Close implements Executor Close interface.
func (e *DistinctExec) Close() error {
	return e.Src.Close()
}
