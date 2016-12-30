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

package plan

import (
	"fmt"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/util/types"
)

// TableRange represents a range of row handle.
type TableRange struct {
	LowVal  int64
	HighVal int64
}

// TableScan represents a table scan plan.
type TableScan struct {
	basePlan

	Table  *model.TableInfo
	Desc   bool
	Ranges []TableRange

	// RefAccess indicates it references a previous joined table, used in explain.
	RefAccess bool

	// AccessConditions can be used to build index range.
	AccessConditions []ast.ExprNode

	// FilterConditions can be used to filter result.
	FilterConditions []ast.ExprNode
}

// Accept implements Plan Accept interface.
func (p *TableScan) Accept(v Visitor) (Plan, bool) {
	np, _ := v.Enter(p)
	return v.Leave(np)
}

// ShowDDL is for showing DDL information.
type ShowDDL struct {
	basePlan
}

// Accept implements Plan Accept interface.
func (p *ShowDDL) Accept(v Visitor) (Plan, bool) {
	np, _ := v.Enter(p)
	return v.Leave(np)
}

// CheckTable is for checking table data.
type CheckTable struct {
	basePlan

	Tables []*ast.TableName
}

// Accept implements Plan Accept interface.
func (p *CheckTable) Accept(v Visitor) (Plan, bool) {
	np, _ := v.Enter(p)
	return v.Leave(np)

}

// IndexRange represents an index range to be scanned.
type IndexRange struct {
	LowVal      []types.Datum
	LowExclude  bool
	HighVal     []types.Datum
	HighExclude bool
}

// IsPoint returns if the index range is a point.
func (ir *IndexRange) IsPoint() bool {
	if len(ir.LowVal) != len(ir.HighVal) {
		return false
	}
	for i := range ir.LowVal {
		a := ir.LowVal[i]
		b := ir.HighVal[i]
		if a.Kind() == types.KindMinNotNull || b.Kind() == types.KindMaxValue {
			return false
		}
		cmp, err := a.CompareDatum(b)
		if err != nil {
			return false
		}
		if cmp != 0 {
			return false
		}
	}
	return !ir.LowExclude && !ir.HighExclude
}

// IndexScan represents an index scan plan.
type IndexScan struct {
	basePlan

	// The index used.
	Index *model.IndexInfo

	// The table to lookup.
	Table *model.TableInfo

	// Ordered and non-overlapping ranges to be scanned.
	Ranges []*IndexRange

	// Desc indicates whether the index should be scanned in descending order.
	Desc bool

	// RefAccess indicates it references a previous joined table, used in explain.
	RefAccess bool

	// AccessConditions can be used to build index range.
	AccessConditions []ast.ExprNode

	// Number of leading equal access condition.
	// The offset of each equal condition correspond to the offset of index column.
	// For example, an index has column (a, b, c), condition is 'a = 0 and b = 0 and c > 0'
	// AccessEqualCount would be 2.
	AccessEqualCount int

	// FilterConditions can be used to filter result.
	FilterConditions []ast.ExprNode
}

// Accept implements Plan Accept interface.
func (p *IndexScan) Accept(v Visitor) (Plan, bool) {
	np, _ := v.Enter(p)
	return v.Leave(np)
}

// JoinOuter represents outer join plan.
type JoinOuter struct {
	basePlan

	Outer Plan
	Inner Plan
}

// Accept implements Plan interface.
func (p *JoinOuter) Accept(v Visitor) (Plan, bool) {
	np, skip := v.Enter(p)
	if skip {
		return v.Leave(np)
	}
	p = np.(*JoinOuter)
	var ok bool
	p.Outer, ok = p.Outer.Accept(v)
	if !ok {
		return p, false
	}
	p.Inner, ok = p.Inner.Accept(v)
	if !ok {
		return p, false
	}
	return v.Leave(p)
}

// JoinInner represents inner join plan.
type JoinInner struct {
	basePlan

	Inners     []Plan
	Conditions []ast.ExprNode
}

func (p *JoinInner) String() string {
	return fmt.Sprintf("JoinInner()")
}

// Accept implements Plan interface.
func (p *JoinInner) Accept(v Visitor) (Plan, bool) {
	np, skip := v.Enter(p)
	if skip {
		return v.Leave(np)
	}
	p = np.(*JoinInner)
	for i, in := range p.Inners {
		x, ok := in.Accept(v)
		if !ok {
			return p, false
		}
		p.Inners[i] = x
	}
	return v.Leave(p)
}

// SelectLock represents a select lock plan.
type SelectLock struct {
	planWithSrc

	Lock ast.SelectLockType
}

// Accept implements Plan Accept interface.
func (p *SelectLock) Accept(v Visitor) (Plan, bool) {
	np, skip := v.Enter(p)
	if skip {
		return v.Leave(np)
	}
	p = np.(*SelectLock)
	var ok bool
	p.src, ok = p.src.Accept(v)
	if !ok {
		return p, false
	}
	return v.Leave(p)
}

// SetLimit implements Plan SetLimit interface.
func (p *SelectLock) SetLimit(limit float64) {
	p.limit = limit
	p.src.SetLimit(p.limit)
}

// SelectFields represents a select fields plan.
type SelectFields struct {
	planWithSrc
}

// Accept implements Plan Accept interface.
func (p *SelectFields) Accept(v Visitor) (Plan, bool) {
	np, skip := v.Enter(p)
	if skip {
		return v.Leave(np)
	}
	p = np.(*SelectFields)
	if p.src != nil {
		var ok bool
		p.src, ok = p.src.Accept(v)
		if !ok {
			return p, false
		}
	}
	return v.Leave(p)
}

// SetLimit implements Plan SetLimit interface.
func (p *SelectFields) SetLimit(limit float64) {
	p.limit = limit
	if p.src != nil {
		p.src.SetLimit(limit)
	}
}

// Sort represents a sorting plan.
type Sort struct {
	planWithSrc

	ByItems []*ast.ByItem
}

// Accept implements Plan Accept interface.
func (p *Sort) Accept(v Visitor) (Plan, bool) {
	np, skip := v.Enter(p)
	if skip {
		return v.Leave(np)
	}
	p = np.(*Sort)
	var ok bool
	p.src, ok = p.src.Accept(v)
	if !ok {
		return p, false
	}
	return v.Leave(p)
}

// SetLimit implements Plan SetLimit interface.
// It set the Src limit only if it is bypassed.
// Bypass has to be determined before this get called.
func (p *Sort) SetLimit(limit float64) {
	p.limit = limit
}

// Limit represents offset and limit plan.
type Limit struct {
	planWithSrc

	Offset uint64
	Count  uint64
}

// Accept implements Plan Accept interface.
func (p *Limit) Accept(v Visitor) (Plan, bool) {
	np, skip := v.Enter(p)
	if skip {
		return v.Leave(np)
	}
	p = np.(*Limit)
	var ok bool
	p.src, ok = p.src.Accept(v)
	if !ok {
		return p, false
	}
	return v.Leave(p)
}

// SetLimit implements Plan SetLimit interface.
// As Limit itself determine the real limit,
// We just ignore the input, and set the real limit.
func (p *Limit) SetLimit(limit float64) {
	p.limit = float64(p.Offset + p.Count)
	p.src.SetLimit(p.limit)
}

// Union represents Union plan.
type Union struct {
	basePlan

	Selects []Plan
}

// Accept implements Plan Accept interface.
func (p *Union) Accept(v Visitor) (Plan, bool) {
	np, skip := v.Enter(p)
	if skip {
		return v.Leave(p)
	}
	p = np.(*Union)
	for i, sel := range p.Selects {
		var ok bool
		p.Selects[i], ok = sel.Accept(v)
		if !ok {
			return p, false
		}
	}
	return v.Leave(p)
}

// Distinct represents Distinct plan.
type Distinct struct {
	planWithSrc
}

// Accept implements Plan Accept interface.
func (p *Distinct) Accept(v Visitor) (Plan, bool) {
	np, skip := v.Enter(p)
	if skip {
		return v.Leave(p)
	}
	p = np.(*Distinct)
	var ok bool
	p.src, ok = p.src.Accept(v)
	if !ok {
		return p, false
	}
	return v.Leave(p)
}

// SetLimit implements Plan SetLimit interface.
func (p *Distinct) SetLimit(limit float64) {
	p.limit = limit
	if p.src != nil {
		p.src.SetLimit(limit)
	}
}

// Prepare represents prepare plan.
type Prepare struct {
	basePlan

	Name    string
	SQLText string
}

// Accept implements Plan Accept interface.
func (p *Prepare) Accept(v Visitor) (Plan, bool) {
	np, skip := v.Enter(p)
	if skip {
		return v.Leave(np)
	}
	p = np.(*Prepare)
	return v.Leave(p)
}

// Execute represents prepare plan.
type Execute struct {
	basePlan

	Name      string
	UsingVars []ast.ExprNode
	ID        uint32
}

// Accept implements Plan Accept interface.
func (p *Execute) Accept(v Visitor) (Plan, bool) {
	np, skip := v.Enter(p)
	if skip {
		return v.Leave(np)
	}
	p = np.(*Execute)
	return v.Leave(p)
}

// Deallocate represents deallocate plan.
type Deallocate struct {
	basePlan

	Name string
}

// Accept implements Plan Accept interface.
func (p *Deallocate) Accept(v Visitor) (Plan, bool) {
	np, skip := v.Enter(p)
	if skip {
		return v.Leave(np)
	}
	p = np.(*Deallocate)
	return v.Leave(p)
}

// Aggregate represents a select fields plan.
type Aggregate struct {
	planWithSrc
	AggFuncs     []*ast.AggregateFuncExpr
	GroupByItems []*ast.ByItem
}

// Accept implements Plan Accept interface.
func (p *Aggregate) Accept(v Visitor) (Plan, bool) {
	np, skip := v.Enter(p)
	if skip {
		return v.Leave(np)
	}
	p = np.(*Aggregate)
	if p.src != nil {
		var ok bool
		p.src, ok = p.src.Accept(v)
		if !ok {
			return p, false
		}
	}
	return v.Leave(p)
}

// SetLimit implements Plan SetLimit interface.
func (p *Aggregate) SetLimit(limit float64) {
	p.limit = limit
	if p.src != nil {
		p.src.SetLimit(limit)
	}
}

// Having represents a having plan.
// The having plan should after aggregate plan.
type Having struct {
	planWithSrc

	// Originally the WHERE or ON condition is parsed into a single expression,
	// but after we converted to CNF(Conjunctive normal form), it can be
	// split into a list of AND conditions.
	Conditions []ast.ExprNode
}

// Accept implements Plan Accept interface.
func (p *Having) Accept(v Visitor) (Plan, bool) {
	np, skip := v.Enter(p)
	if skip {
		return v.Leave(np)
	}
	p = np.(*Having)
	var ok bool
	p.src, ok = p.src.Accept(v)
	if !ok {
		return p, false
	}
	return v.Leave(p)
}

// SetLimit implements Plan SetLimit interface.
func (p *Having) SetLimit(limit float64) {
	p.limit = limit
	// We assume 50% of the src row is filtered out.
	p.src.SetLimit(limit * 2)
}

// Update represents an update plan.
type Update struct {
	basePlan

	OrderedList []*ast.Assignment // OrderedList has the same offset as TablePlan's result fields.
	SelectPlan  Plan
}

// Accept implements Plan Accept interface.
func (p *Update) Accept(v Visitor) (Plan, bool) {
	np, skip := v.Enter(p)
	if skip {
		return v.Leave(np)
	}
	p = np.(*Update)
	var ok bool
	p.SelectPlan, ok = p.SelectPlan.Accept(v)
	if !ok {
		return p, false
	}
	return v.Leave(p)
}

// Delete represents a delete plan.
type Delete struct {
	basePlan

	SelectPlan   Plan
	Tables       []*ast.TableName
	IsMultiTable bool
}

// Accept implements Plan Accept interface.
func (p *Delete) Accept(v Visitor) (Plan, bool) {
	np, skip := v.Enter(p)
	if skip {
		return v.Leave(np)
	}
	p = np.(*Delete)
	var ok bool
	p.SelectPlan, ok = p.SelectPlan.Accept(v)
	if !ok {
		return p, false
	}
	return v.Leave(p)
}

// Filter represents a plan that filter srcplan result.
type Filter struct {
	planWithSrc

	// Originally the WHERE or ON condition is parsed into a single expression,
	// but after we converted to CNF(Conjunctive normal form), it can be
	// split into a list of AND conditions.
	Conditions []ast.ExprNode
}

// Accept implements Plan Accept interface.
func (p *Filter) Accept(v Visitor) (Plan, bool) {
	np, skip := v.Enter(p)
	if skip {
		return v.Leave(np)
	}
	p = np.(*Filter)
	var ok bool
	p.src, ok = p.src.Accept(v)
	if !ok {
		return p, false
	}
	return v.Leave(p)
}

// SetLimit implements Plan SetLimit interface.
func (p *Filter) SetLimit(limit float64) {
	p.limit = limit
	// We assume 50% of the src row is filtered out.
	p.src.SetLimit(limit * 2)
}

// Show represents a show plan.
type Show struct {
	basePlan

	Tp     ast.ShowStmtType // Databases/Tables/Columns/....
	DBName string
	Table  *ast.TableName  // Used for showing columns.
	Column *ast.ColumnName // Used for `desc table column`.
	Flag   int             // Some flag parsed from sql, such as FULL.
	Full   bool
	User   string // Used for show grants.

	// Used by show variables
	GlobalScope bool
}

// Accept implements Plan Accept interface.
func (p *Show) Accept(v Visitor) (Plan, bool) {
	np, skip := v.Enter(p)
	if skip {
		return v.Leave(np)
	}
	p = np.(*Show)
	return v.Leave(p)
}

// Simple represents a simple statement plan which doesn't need any optimization.
type Simple struct {
	basePlan

	Statement ast.StmtNode
}

// Accept implements Plan Accept interface.
func (p *Simple) Accept(v Visitor) (Plan, bool) {
	np, skip := v.Enter(p)
	if skip {
		return v.Leave(np)
	}
	p = np.(*Simple)
	return v.Leave(p)
}

// Insert represents an insert plan.
type Insert struct {
	basePlan

	Table       *ast.TableRefsClause
	Columns     []*ast.ColumnName
	Lists       [][]ast.ExprNode
	Setlist     []*ast.Assignment
	OnDuplicate []*ast.Assignment
	SelectPlan  Plan

	IsReplace bool
	Priority  int
}

// Accept implements Plan Accept interface.
func (p *Insert) Accept(v Visitor) (Plan, bool) {
	np, skip := v.Enter(p)
	if skip {
		return v.Leave(np)
	}
	p = np.(*Insert)
	if p.SelectPlan != nil {
		var ok bool
		p.SelectPlan, ok = p.SelectPlan.Accept(v)
		if !ok {
			return p, false
		}
	}
	return v.Leave(p)
}

// DDL represents a DDL statement plan.
type DDL struct {
	basePlan

	Statement ast.DDLNode
}

// Accept implements Plan Accept interface.
func (p *DDL) Accept(v Visitor) (Plan, bool) {
	np, skip := v.Enter(p)
	if skip {
		return v.Leave(np)
	}
	p = np.(*DDL)
	return v.Leave(p)
}

// Explain represents a explain plan.
type Explain struct {
	basePlan

	StmtPlan Plan
}

// Accept implements Plan Accept interface.
func (p *Explain) Accept(v Visitor) (Plan, bool) {
	np, skip := v.Enter(p)
	if skip {
		v.Leave(np)
	}
	p = np.(*Explain)
	return v.Leave(p)
}
