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
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/evaluator"
	"github.com/pingcap/tidb/infoschema"
	"github.com/pingcap/tidb/optimizer"
	"github.com/pingcap/tidb/optimizer/plan"
	"github.com/pingcap/tidb/parser"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/sessionctx/variable"
)

var (
	_ Executor = &DeallocateExec{}
	_ Executor = &ExecuteExec{}
	_ Executor = &PrepareExec{}
)

type paramMarkerSorter struct {
	markers []*ast.ParamMarkerExpr
}

func (p *paramMarkerSorter) Len() int {
	return len(p.markers)
}

func (p *paramMarkerSorter) Less(i, j int) bool {
	return p.markers[i].Offset < p.markers[j].Offset
}

func (p *paramMarkerSorter) Swap(i, j int) {
	p.markers[i], p.markers[j] = p.markers[j], p.markers[i]
}

type paramMarkerExtractor struct {
	markers []*ast.ParamMarkerExpr
}

func (e *paramMarkerExtractor) Enter(in ast.Node) (ast.Node, bool) {
	return in, false
}

func (e *paramMarkerExtractor) Leave(in ast.Node) (ast.Node, bool) {
	if x, ok := in.(*ast.ParamMarkerExpr); ok {
		e.markers = append(e.markers, x)
	}
	return in, true
}

// Prepared represents a prepared statement.
type Prepared struct {
	Stmt          ast.StmtNode
	Params        []*ast.ParamMarkerExpr
	SchemaVersion int64
}

// PrepareExec represents a PREPARE executor.
type PrepareExec struct {
	IS      infoschema.InfoSchema
	Ctx     context.Context
	Name    string
	SQLText string

	ID           uint32
	ResultFields []*ast.ResultField
	ParamCount   int
	Err          error
}

// Fields implements Executor Fields interface.
func (e *PrepareExec) Fields() []*ast.ResultField {
	// returns nil to indicate prepare will not return Recordset.
	return nil
}

// Next implements Executor Next interface.
func (e *PrepareExec) Next() (*Row, error) {
	e.DoPrepare()
	return nil, e.Err
}

// Close implements plan.Plan Close interface.
func (e *PrepareExec) Close() error {
	return nil
}

// DoPrepare prepares the statement, it can be called multiple times without
// side effect.
func (e *PrepareExec) DoPrepare() {
	vars := variable.GetSessionVars(e.Ctx)
	if e.ID != 0 {
		// Must be the case when we retry a prepare.
		// Make sure it is idempotent.
		_, ok := vars.PreparedStmts[e.ID]
		if ok {
			return
		}
	}
	charset, collation := variable.GetCharsetInfo(e.Ctx)
	stmts, err := parser.Parse(e.SQLText, charset, collation)
	if err != nil {
		e.Err = errors.Trace(err)
		return
	}
	if len(stmts) != 1 {
		e.Err = ErrPrepareMulti
		return
	}
	stmt := stmts[0]
	var extractor paramMarkerExtractor
	stmt.Accept(&extractor)

	// The parameter markers are appended in visiting order, which may not
	// be the same as the position order in the query string. We need to
	// sort it by position.
	sorter := &paramMarkerSorter{markers: extractor.markers}
	sort.Sort(sorter)
	e.ParamCount = len(sorter.markers)
	prepared := &Prepared{
		Stmt:          stmt,
		Params:        sorter.markers,
		SchemaVersion: e.IS.SchemaMetaVersion(),
	}

	err = optimizer.Prepare(e.IS, e.Ctx, stmt)
	if err != nil {
		e.Err = errors.Trace(err)
		return
	}
	if resultSetNode, ok := stmt.(ast.ResultSetNode); ok {
		e.ResultFields = resultSetNode.GetResultFields()
	}

	if e.ID == 0 {
		e.ID = vars.GetNextPreparedStmtID()
	}
	if e.Name != "" {
		vars.PreparedStmtNameToID[e.Name] = e.ID
	}
	vars.PreparedStmts[e.ID] = prepared
}

// ExecuteExec represents an EXECUTE executor.
// It executes a prepared statement.
type ExecuteExec struct {
	IS        infoschema.InfoSchema
	Ctx       context.Context
	Name      string
	UsingVars []ast.ExprNode
	ID        uint32
	StmtExec  Executor
}

// Fields implements Executor Fields interface.
func (e *ExecuteExec) Fields() []*ast.ResultField {
	// Will never be called.
	return nil
}

// Next implements Executor Next interface.
func (e *ExecuteExec) Next() (*Row, error) {
	// Will never be called.
	return nil, nil
}

// Close implements plan.Plan Close interface.
func (e *ExecuteExec) Close() error {
	// Will never be called.
	return nil
}

// Build builds a prepared statement into an executor.
func (e *ExecuteExec) Build() error {
	vars := variable.GetSessionVars(e.Ctx)
	if e.Name != "" {
		e.ID = vars.PreparedStmtNameToID[e.Name]
	}
	v := vars.PreparedStmts[e.ID]
	if v == nil {
		return ErrStmtNotFound
	}
	prepared := v.(*Prepared)

	if len(prepared.Params) != len(e.UsingVars) {
		return ErrWrongParamCount
	}

	for i, usingVar := range e.UsingVars {
		val, err := evaluator.Eval(e.Ctx, usingVar)
		if err != nil {
			return errors.Trace(err)
		}
		prepared.Params[i].SetValue(val)
	}

	if prepared.SchemaVersion != e.IS.SchemaMetaVersion() {
		// If the schema version has changed we need to prepare it again,
		// if this time it failed, the real reason for the error is schema changed.
		err := optimizer.Prepare(e.IS, e.Ctx, prepared.Stmt)
		if err != nil {
			return ErrSchemaChanged.Gen("Schema change casued error: %s", err.Error())
		}
		prepared.SchemaVersion = e.IS.SchemaMetaVersion()
	}
	sb := &subqueryBuilder{is: e.IS}
	plan, err := optimizer.Optimize(e.Ctx, prepared.Stmt, sb)
	if err != nil {
		return errors.Trace(err)
	}
	b := newExecutorBuilder(e.Ctx, e.IS)
	stmtExec := b.build(plan)
	if b.err != nil {
		return errors.Trace(b.err)
	}
	e.StmtExec = stmtExec
	return nil
}

// DeallocateExec represent a DEALLOCATE executor.
type DeallocateExec struct {
	Name string
	ctx  context.Context
}

// Fields implements Executor Fields interface.
func (e *DeallocateExec) Fields() []*ast.ResultField {
	return nil
}

// Next implements Executor Next interface.
func (e *DeallocateExec) Next() (*Row, error) {
	vars := variable.GetSessionVars(e.ctx)
	id, ok := vars.PreparedStmtNameToID[e.Name]
	if !ok {
		return nil, ErrStmtNotFound
	}
	delete(vars.PreparedStmtNameToID, e.Name)
	delete(vars.PreparedStmts, id)
	return nil, nil
}

// Close implements plan.Plan Close interface.
func (e *DeallocateExec) Close() error {
	return nil
}

// CompileExecutePreparedStmt compiles a session Execute command to a stmt.Statement.
func CompileExecutePreparedStmt(ctx context.Context, ID uint32, args ...interface{}) ast.Statement {
	execPlan := &plan.Execute{ID: ID}
	execPlan.UsingVars = make([]ast.ExprNode, len(args))
	for i, val := range args {
		execPlan.UsingVars[i] = ast.NewValueExpr(val)
	}
	sa := &statement{
		is:   sessionctx.GetDomain(ctx).InfoSchema(),
		plan: execPlan,
	}
	return sa
}
