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

package optimizer

import (
	"fmt"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/column"
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/infoschema"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/sessionctx/db"
	"github.com/pingcap/tidb/util/types"
)

// ResolveName resolves table name and column name.
// It generates ResultFields for ResultSetNode and resolves ColumnNameExpr to a ResultField.
func ResolveName(node ast.Node, info infoschema.InfoSchema, ctx context.Context) error {
	defaultSchema := db.GetCurrentSchema(ctx)
	resolver := nameResolver{Info: info, Ctx: ctx, DefaultSchema: model.NewCIStr(defaultSchema)}
	node.Accept(&resolver)
	return errors.Trace(resolver.Err)
}

// nameResolver is the visitor to resolve table name and column name.
// In general, a reference can only refer to information that are available for it.
// So children elements are visited in the order that previous elements make information
// available for following elements.
//
// During visiting, information are collected and stored in resolverContext.
// When we enter a subquery, a new resolverContext is pushed to the contextStack, so subquery
// information can overwrite outer query information. When we look up for a column reference,
// we look up from top to bottom in the contextStack.
type nameResolver struct {
	Info            infoschema.InfoSchema
	Ctx             context.Context
	DefaultSchema   model.CIStr
	Err             error
	useOuterContext bool

	contextStack []*resolverContext
}

// resolverContext stores information in a single level of select statement
// that table name and column name can be resolved.
type resolverContext struct {
	/* For Select Statement. */
	// table map to lookup and check table name conflict.
	tableMap map[string]int
	// table map to lookup and check derived-table(subselect) name conflict.
	derivedTableMap map[string]int
	// tableSources collected in from clause.
	tables []*ast.TableSource
	// result fields collected in select field list.
	fieldList []*ast.ResultField
	// result fields collected in group by clause.
	groupBy []*ast.ResultField

	// The join node stack is used by on condition to find out
	// available tables to reference. On condition can only
	// refer to tables involved in current join.
	joinNodeStack []*ast.Join

	// When visiting TableRefs, tables in this context are not available
	// because it is being collected.
	inTableRefs bool
	// When visiting on conditon only tables in current join node are available.
	inOnCondition bool
	// When visiting field list, fieldList in this context are not available.
	inFieldList bool
	// When visiting group by, groupBy fields are not available.
	inGroupBy bool
	// When visiting having, only fieldList and groupBy fields are available.
	inHaving bool
	// When visiting having, checks if the expr is an aggregate function expr.
	inHavingAgg bool
	// OrderBy clause has different resolving rule than group by.
	inOrderBy bool
	// When visiting column name in ByItem, we should know if the column name is in an expression.
	inByItemExpression bool
	// If subquery use outer context.
	useOuterContext bool
	// When visiting multi-table delete stmt table list.
	inDeleteTableList bool
	// When visiting create/drop table statement.
	inCreateOrDropTable bool
	// When visiting show statement.
	inShow bool
}

// currentContext gets the current resolverContext.
func (nr *nameResolver) currentContext() *resolverContext {
	stackLen := len(nr.contextStack)
	if stackLen == 0 {
		return nil
	}
	return nr.contextStack[stackLen-1]
}

// pushContext is called when we enter a statement.
func (nr *nameResolver) pushContext() {
	nr.contextStack = append(nr.contextStack, &resolverContext{
		tableMap:        map[string]int{},
		derivedTableMap: map[string]int{},
	})
}

// popContext is called when we leave a statement.
func (nr *nameResolver) popContext() {
	nr.contextStack = nr.contextStack[:len(nr.contextStack)-1]
}

// pushJoin is called when we enter a join node.
func (nr *nameResolver) pushJoin(j *ast.Join) {
	ctx := nr.currentContext()
	ctx.joinNodeStack = append(ctx.joinNodeStack, j)
}

// popJoin is called when we leave a join node.
func (nr *nameResolver) popJoin() {
	ctx := nr.currentContext()
	ctx.joinNodeStack = ctx.joinNodeStack[:len(ctx.joinNodeStack)-1]
}

// Enter implements ast.Visitor interface.
func (nr *nameResolver) Enter(inNode ast.Node) (outNode ast.Node, skipChildren bool) {
	switch v := inNode.(type) {
	case *ast.AdminStmt:
		nr.pushContext()
	case *ast.AggregateFuncExpr:
		ctx := nr.currentContext()
		if ctx.inHaving {
			ctx.inHavingAgg = true
		}
	case *ast.AlterTableStmt:
		nr.pushContext()
	case *ast.ByItem:
		if _, ok := v.Expr.(*ast.ColumnNameExpr); !ok {
			// If ByItem is not a single column name expression,
			// the resolving rule is different from order by clause.
			nr.currentContext().inByItemExpression = true
		}
		if nr.currentContext().inGroupBy {
			// make sure item is not aggregate function
			if ast.HasAggFlag(v.Expr) {
				nr.Err = ErrInvalidGroupFuncUse
				return inNode, true
			}
		}
	case *ast.CreateIndexStmt:
		nr.pushContext()
	case *ast.CreateTableStmt:
		nr.pushContext()
		nr.currentContext().inCreateOrDropTable = true
	case *ast.DeleteStmt:
		nr.pushContext()
	case *ast.DeleteTableList:
		nr.currentContext().inDeleteTableList = true
	case *ast.DoStmt:
		nr.pushContext()
	case *ast.DropTableStmt:
		nr.pushContext()
		nr.currentContext().inCreateOrDropTable = true
	case *ast.DropIndexStmt:
		nr.pushContext()
	case *ast.FieldList:
		nr.currentContext().inFieldList = true
	case *ast.GroupByClause:
		nr.currentContext().inGroupBy = true
	case *ast.HavingClause:
		nr.currentContext().inHaving = true
	case *ast.InsertStmt:
		nr.pushContext()
	case *ast.Join:
		nr.pushJoin(v)
	case *ast.OnCondition:
		nr.currentContext().inOnCondition = true
	case *ast.OrderByClause:
		nr.currentContext().inOrderBy = true
	case *ast.SelectStmt:
		nr.pushContext()
	case *ast.SetStmt:
		for _, assign := range v.Variables {
			if cn, ok := assign.Value.(*ast.ColumnNameExpr); ok && cn.Name.Table.L == "" {
				// Convert column name expression to string value expression.
				assign.Value = ast.NewValueExpr(cn.Name.Name.O)
			}
		}
		nr.pushContext()
	case *ast.ShowStmt:
		nr.pushContext()
		nr.currentContext().inShow = true
		nr.fillShowFields(v)
	case *ast.TableRefsClause:
		nr.currentContext().inTableRefs = true
	case *ast.TruncateTableStmt:
		nr.pushContext()
	case *ast.UnionStmt:
		nr.pushContext()
	case *ast.UpdateStmt:
		nr.pushContext()
	}
	return inNode, false
}

// Leave implements ast.Visitor interface.
func (nr *nameResolver) Leave(inNode ast.Node) (node ast.Node, ok bool) {
	switch v := inNode.(type) {
	case *ast.AdminStmt:
		nr.popContext()
	case *ast.AggregateFuncExpr:
		ctx := nr.currentContext()
		if ctx.inHaving {
			ctx.inHavingAgg = false
		}
	case *ast.AlterTableStmt:
		nr.popContext()
	case *ast.TableName:
		nr.handleTableName(v)
	case *ast.ColumnNameExpr:
		nr.handleColumnName(v)
	case *ast.CreateIndexStmt:
		nr.popContext()
	case *ast.CreateTableStmt:
		nr.popContext()
	case *ast.DeleteTableList:
		nr.currentContext().inDeleteTableList = false
	case *ast.DoStmt:
		nr.popContext()
	case *ast.DropIndexStmt:
		nr.popContext()
	case *ast.DropTableStmt:
		nr.popContext()
	case *ast.TableSource:
		nr.handleTableSource(v)
	case *ast.OnCondition:
		nr.currentContext().inOnCondition = false
	case *ast.Join:
		nr.handleJoin(v)
		nr.popJoin()
	case *ast.TableRefsClause:
		nr.currentContext().inTableRefs = false
	case *ast.FieldList:
		nr.handleFieldList(v)
		nr.currentContext().inFieldList = false
	case *ast.GroupByClause:
		ctx := nr.currentContext()
		ctx.inGroupBy = false
		for _, item := range v.Items {
			switch x := item.Expr.(type) {
			case *ast.ColumnNameExpr:
				ctx.groupBy = append(ctx.groupBy, x.Refer)
			}
		}
	case *ast.HavingClause:
		nr.currentContext().inHaving = false
	case *ast.OrderByClause:
		nr.currentContext().inOrderBy = false
	case *ast.ByItem:
		nr.currentContext().inByItemExpression = false
	case *ast.PositionExpr:
		nr.handlePosition(v)
	case *ast.SelectStmt:
		ctx := nr.currentContext()
		v.SetResultFields(ctx.fieldList)
		if ctx.useOuterContext {
			nr.useOuterContext = true
		}
		nr.popContext()
	case *ast.SetStmt:
		nr.popContext()
	case *ast.ShowStmt:
		nr.popContext()
	case *ast.SubqueryExpr:
		if nr.useOuterContext {
			// TODO: check this
			// If there is a deep nest of subquery, there may be something wrong.
			v.UseOuterContext = true
			nr.useOuterContext = false
		}
	case *ast.TruncateTableStmt:
		nr.popContext()
	case *ast.UnionStmt:
		ctx := nr.currentContext()
		v.SetResultFields(ctx.fieldList)
		if ctx.useOuterContext {
			nr.useOuterContext = true
		}
		nr.popContext()
	case *ast.UnionSelectList:
		nr.handleUnionSelectList(v)
	case *ast.InsertStmt:
		nr.popContext()
	case *ast.DeleteStmt:
		nr.popContext()
	case *ast.UpdateStmt:
		nr.popContext()
	}
	return inNode, nr.Err == nil
}

// handleTableName looks up and sets the schema information and result fields for table name.
func (nr *nameResolver) handleTableName(tn *ast.TableName) {
	if tn.Schema.L == "" {
		tn.Schema = nr.DefaultSchema
	}
	ctx := nr.currentContext()
	if ctx.inCreateOrDropTable {
		// The table may not exist in create table or drop table statement.
		// Skip resolving the table to avoid error.
		return
	}
	if ctx.inDeleteTableList {
		idx, ok := ctx.tableMap[nr.tableUniqueName(tn.Schema, tn.Name)]
		if !ok {
			nr.Err = errors.Errorf("Unknown table %s", tn.Name.O)
			return
		}
		ts := ctx.tables[idx]
		tableName := ts.Source.(*ast.TableName)
		tn.DBInfo = tableName.DBInfo
		tn.TableInfo = tableName.TableInfo
		tn.SetResultFields(tableName.GetResultFields())
		return
	}
	table, err := nr.Info.TableByName(tn.Schema, tn.Name)
	if err != nil {
		nr.Err = errors.Trace(err)
		return
	}
	tn.TableInfo = table.Meta()
	dbInfo, _ := nr.Info.SchemaByName(tn.Schema)
	tn.DBInfo = dbInfo

	rfs := make([]*ast.ResultField, 0, len(tn.TableInfo.Columns))
	for _, v := range tn.TableInfo.Columns {
		if v.State != model.StatePublic {
			continue
		}
		expr := &ast.ValueExpr{}
		expr.SetType(&v.FieldType)
		rf := &ast.ResultField{
			Column:    v,
			Table:     tn.TableInfo,
			DBName:    tn.Schema,
			Expr:      expr,
			TableName: tn,
		}
		rfs = append(rfs, rf)
	}
	tn.SetResultFields(rfs)
	return
}

// handleTableSources checks name duplication
// and puts the table source in current resolverContext.
// Note:
// "select * from t as a join (select 1) as a;" is not duplicate.
// "select * from t as a join t as a;" is duplicate.
// "select * from (select 1) as a join (select 1) as a;" is duplicate.
func (nr *nameResolver) handleTableSource(ts *ast.TableSource) {
	for _, v := range ts.GetResultFields() {
		v.TableAsName = ts.AsName
	}
	ctx := nr.currentContext()
	switch ts.Source.(type) {
	case *ast.TableName:
		var name string
		if ts.AsName.L != "" {
			name = ts.AsName.L
		} else {
			tableName := ts.Source.(*ast.TableName)
			name = nr.tableUniqueName(tableName.Schema, tableName.Name)
		}
		if _, ok := ctx.tableMap[name]; ok {
			nr.Err = errors.Errorf("duplicated table/alias name %s", name)
			return
		}
		ctx.tableMap[name] = len(ctx.tables)
	case *ast.SelectStmt:
		name := ts.AsName.L
		if _, ok := ctx.derivedTableMap[name]; ok {
			nr.Err = errors.Errorf("duplicated table/alias name %s", name)
			return
		}
		ctx.derivedTableMap[name] = len(ctx.tables)
	}
	dupNames := make(map[string]struct{}, len(ts.GetResultFields()))
	for _, f := range ts.GetResultFields() {
		// duplicate column name in one table is not allowed.
		// "select * from (select 1, 1) as a;" is duplicate.
		name := f.ColumnAsName.L
		if name == "" {
			name = f.Column.Name.L
		}
		if _, ok := dupNames[name]; ok {
			nr.Err = errors.Errorf("Duplicate column name '%s'", name)
			return
		}
		dupNames[name] = struct{}{}
	}
	ctx.tables = append(ctx.tables, ts)
	return
}

// handleJoin sets result fields for join.
func (nr *nameResolver) handleJoin(j *ast.Join) {
	if j.Right == nil {
		j.SetResultFields(j.Left.GetResultFields())
		return
	}
	leftLen := len(j.Left.GetResultFields())
	rightLen := len(j.Right.GetResultFields())
	rfs := make([]*ast.ResultField, leftLen+rightLen)
	copy(rfs, j.Left.GetResultFields())
	copy(rfs[leftLen:], j.Right.GetResultFields())
	j.SetResultFields(rfs)
}

// handleColumnName looks up and sets ResultField for
// the column name.
func (nr *nameResolver) handleColumnName(cn *ast.ColumnNameExpr) {
	ctx := nr.currentContext()
	if ctx.inOnCondition {
		// In on condition, only tables within current join is available.
		nr.resolveColumnNameInOnCondition(cn)
		return
	}

	// Try to resolve the column name form top to bottom in the context stack.
	for i := len(nr.contextStack) - 1; i >= 0; i-- {
		if nr.resolveColumnNameInContext(nr.contextStack[i], cn) {
			// Column is already resolved or encountered an error.
			if i < len(nr.contextStack)-1 {
				// If in subselect, the query use outer query.
				nr.currentContext().useOuterContext = true
			}
			return
		}
	}
	nr.Err = errors.Errorf("unknown column %s", cn.Name.Name.L)
}

// resolveColumnNameInContext looks up and sets ResultField for a column with the ctx.
func (nr *nameResolver) resolveColumnNameInContext(ctx *resolverContext, cn *ast.ColumnNameExpr) bool {
	if ctx.inTableRefs {
		// In TableRefsClause, column reference only in join on condition which is handled before.
		return false
	}
	if ctx.inFieldList {
		// only resolve column using tables.
		return nr.resolveColumnInTableSources(cn, ctx.tables)
	}
	if ctx.inGroupBy {
		// From tables first, then field list.
		// If ctx.InByItemExpression is true, the item is not an identifier.
		// Otherwise it is an identifier.
		if ctx.inByItemExpression {
			// From table first, then field list.
			if nr.resolveColumnInTableSources(cn, ctx.tables) {
				return true
			}
			found := nr.resolveColumnInResultFields(ctx, cn, ctx.fieldList)
			if nr.Err == nil && found {
				// Check if resolved refer is an aggregate function expr.
				if _, ok := cn.Refer.Expr.(*ast.AggregateFuncExpr); ok {
					nr.Err = ErrIllegalReference.Gen("Reference '%s' not supported (reference to group function)", cn.Name.Name.O)
				}
			}
			return found
		}
		// Resolve from table first, then from select list.
		found := nr.resolveColumnInTableSources(cn, ctx.tables)
		if nr.Err != nil {
			return found
		}
		// We should copy the refer here.
		// Because if the ByItem is an identifier, we should check if it
		// is ambiguous even it is already resolved from table source.
		// If the ByItem is not an identifier, we do not need the second check.
		r := cn.Refer
		if nr.resolveColumnInResultFields(ctx, cn, ctx.fieldList) {
			if nr.Err != nil {
				return true
			}
			if r != nil {
				// It is not ambiguous and already resolved from table source.
				// We should restore its Refer.
				cn.Refer = r
			}
			if _, ok := cn.Refer.Expr.(*ast.AggregateFuncExpr); ok {
				nr.Err = ErrIllegalReference.Gen("Reference '%s' not supported (reference to group function)", cn.Name.Name.O)
			}
			return true
		}
		return found
	}
	if ctx.inHaving {
		// First group by, then field list.
		if nr.resolveColumnInResultFields(ctx, cn, ctx.groupBy) {
			return true
		}
		if ctx.inHavingAgg {
			// If cn is in an aggregate function in having clause, check tablesource first.
			if nr.resolveColumnInTableSources(cn, ctx.tables) {
				return true
			}
		}
		return nr.resolveColumnInResultFields(ctx, cn, ctx.fieldList)
	}
	if ctx.inOrderBy {
		if nr.resolveColumnInResultFields(ctx, cn, ctx.groupBy) {
			return true
		}
		if ctx.inByItemExpression {
			// From table first, then field list.
			if nr.resolveColumnInTableSources(cn, ctx.tables) {
				return true
			}
			return nr.resolveColumnInResultFields(ctx, cn, ctx.fieldList)
		}
		// Field list first, then from table.
		if nr.resolveColumnInResultFields(ctx, cn, ctx.fieldList) {
			return true
		}
		return nr.resolveColumnInTableSources(cn, ctx.tables)
	}
	if ctx.inShow {
		return nr.resolveColumnInResultFields(ctx, cn, ctx.fieldList)
	}
	// In where clause.
	return nr.resolveColumnInTableSources(cn, ctx.tables)
}

// resolveColumnNameInOnCondition resolves the column name in current join.
func (nr *nameResolver) resolveColumnNameInOnCondition(cn *ast.ColumnNameExpr) {
	ctx := nr.currentContext()
	join := ctx.joinNodeStack[len(ctx.joinNodeStack)-1]
	tableSources := appendTableSources(nil, join)
	if !nr.resolveColumnInTableSources(cn, tableSources) {
		nr.Err = errors.Errorf("unkown column name %s", cn.Name.Name.O)
	}
}

func (nr *nameResolver) resolveColumnInTableSources(cn *ast.ColumnNameExpr, tableSources []*ast.TableSource) (done bool) {
	var matchedResultField *ast.ResultField
	tableNameL := cn.Name.Table.L
	columnNameL := cn.Name.Name.L
	if tableNameL != "" {
		var matchedTable ast.ResultSetNode
		for _, ts := range tableSources {
			if tableNameL == ts.AsName.L {
				// different table name.
				matchedTable = ts
				break
			} else if ts.AsName.L != "" {
				// Table as name shadows table real name.
				continue
			}
			if tn, ok := ts.Source.(*ast.TableName); ok {
				if cn.Name.Schema.L != "" && cn.Name.Schema.L != tn.Schema.L {
					continue
				}
				if tableNameL == tn.Name.L {
					matchedTable = ts
				}
			}
		}
		if matchedTable != nil {
			resultFields := matchedTable.GetResultFields()
			for _, rf := range resultFields {
				if rf.ColumnAsName.L == columnNameL || rf.Column.Name.L == columnNameL {
					// resolve column.
					matchedResultField = rf
					break
				}
			}
		}
	} else {
		for _, ts := range tableSources {
			rfs := ts.GetResultFields()
			for _, rf := range rfs {
				matchAsName := rf.ColumnAsName.L != "" && rf.ColumnAsName.L == columnNameL
				matchColumnName := rf.ColumnAsName.L == "" && rf.Column.Name.L == columnNameL
				if matchAsName || matchColumnName {
					if matchedResultField != nil {
						nr.Err = errors.Errorf("column %s is ambiguous.", cn.Name.Name.O)
						return true
					}
					matchedResultField = rf
				}
			}
		}
	}
	if matchedResultField != nil {
		// Bind column.
		cn.Refer = matchedResultField
		return true
	}
	return false
}

func (nr *nameResolver) resolveColumnInResultFields(ctx *resolverContext, cn *ast.ColumnNameExpr, rfs []*ast.ResultField) bool {
	var matched *ast.ResultField
	for _, rf := range rfs {
		if cn.Name.Table.L != "" {
			// Check table name
			if rf.TableAsName.L != "" {
				if cn.Name.Table.L != rf.TableAsName.L {
					continue
				}
			} else if cn.Name.Table.L != rf.Table.Name.L {
				continue
			}
		}
		matchAsName := cn.Name.Name.L == rf.ColumnAsName.L
		var matchColumnName bool
		if ctx.inHaving {
			matchColumnName = cn.Name.Name.L == rf.Column.Name.L
		} else {
			matchColumnName = rf.ColumnAsName.L == "" && cn.Name.Name.L == rf.Column.Name.L
		}
		if matchAsName || matchColumnName {
			if rf.Column.Name.L == "" {
				// This is not a real table column, resolve it directly.
				cn.Refer = rf
				return true
			}
			if matched == nil {
				matched = rf
			} else {
				sameColumn := matched.TableName == rf.TableName && matched.Column.Name.L == rf.Column.Name.L
				if !sameColumn {
					nr.Err = errors.Errorf("column %s is ambiguous.", cn.Name.Name.O)
					return true
				}
			}
		}
	}
	if matched != nil {
		// If in GroupBy, we clone the ResultField
		if ctx.inGroupBy || ctx.inHaving || ctx.inOrderBy {
			nf := *matched
			expr := matched.Expr
			if cexpr, ok := expr.(*ast.ColumnNameExpr); ok {
				expr = cexpr.Refer.Expr
			}
			nf.Expr = expr
			matched = &nf
		}
		// Bind column.
		cn.Refer = matched
		return true
	}
	return false
}

// handleFieldList expands wild card field and sets fieldList in current context.
func (nr *nameResolver) handleFieldList(fieldList *ast.FieldList) {
	var resultFields []*ast.ResultField
	for _, v := range fieldList.Fields {
		resultFields = append(resultFields, nr.createResultFields(v)...)
	}
	nr.currentContext().fieldList = resultFields
}

func getInnerFromParentheses(expr ast.ExprNode) ast.ExprNode {
	if pexpr, ok := expr.(*ast.ParenthesesExpr); ok {
		return getInnerFromParentheses(pexpr.Expr)
	}
	return expr
}

// createResultFields creates result field list for a single select field.
func (nr *nameResolver) createResultFields(field *ast.SelectField) (rfs []*ast.ResultField) {
	ctx := nr.currentContext()
	if field.WildCard != nil {
		if len(ctx.tables) == 0 {
			nr.Err = errors.New("No table used.")
			return
		}
		tableRfs := []*ast.ResultField{}
		if field.WildCard.Table.L == "" {
			for _, v := range ctx.tables {
				tableRfs = append(tableRfs, v.GetResultFields()...)
			}
		} else {
			name := nr.tableUniqueName(field.WildCard.Schema, field.WildCard.Table)
			tableIdx, ok1 := ctx.tableMap[name]
			derivedTableIdx, ok2 := ctx.derivedTableMap[name]
			if !ok1 && !ok2 {
				nr.Err = errors.Errorf("unknown table %s.", field.WildCard.Table.O)
			}
			if ok1 {
				tableRfs = ctx.tables[tableIdx].GetResultFields()
			}
			if ok2 {
				tableRfs = append(tableRfs, ctx.tables[derivedTableIdx].GetResultFields()...)
			}

		}
		for _, trf := range tableRfs {
			// Convert it to ColumnNameExpr
			cn := &ast.ColumnName{
				Schema: trf.DBName,
				Table:  trf.Table.Name,
				Name:   trf.ColumnAsName,
			}
			cnExpr := &ast.ColumnNameExpr{
				Name:  cn,
				Refer: trf,
			}
			ast.SetFlag(cnExpr)
			cnExpr.SetType(trf.Expr.GetType())
			rf := *trf
			rf.Expr = cnExpr
			rfs = append(rfs, &rf)
		}
		return
	}
	// The column is visited before so it must has been resolved already.
	rf := &ast.ResultField{ColumnAsName: field.AsName}
	innerExpr := getInnerFromParentheses(field.Expr)
	switch v := innerExpr.(type) {
	case *ast.ColumnNameExpr:
		rf.Column = v.Refer.Column
		rf.Table = v.Refer.Table
		rf.DBName = v.Refer.DBName
		rf.TableName = v.Refer.TableName
		rf.Expr = v
	default:
		rf.Column = &model.ColumnInfo{} // Empty column info.
		rf.Table = &model.TableInfo{}   // Empty table info.
		rf.Expr = v
	}
	if field.AsName.L == "" {
		switch x := innerExpr.(type) {
		case *ast.ColumnNameExpr:
			rf.ColumnAsName = model.NewCIStr(x.Name.Name.O)
		case *ast.ValueExpr:
			if innerExpr.Text() != "" {
				rf.ColumnAsName = model.NewCIStr(innerExpr.Text())
			} else {
				rf.ColumnAsName = model.NewCIStr(field.Text())
			}
		default:
			rf.ColumnAsName = model.NewCIStr(field.Text())
		}
	}
	rfs = append(rfs, rf)
	return
}

func appendTableSources(in []*ast.TableSource, resultSetNode ast.ResultSetNode) (out []*ast.TableSource) {
	switch v := resultSetNode.(type) {
	case *ast.TableSource:
		out = append(in, v)
	case *ast.Join:
		out = appendTableSources(in, v.Left)
		if v.Right != nil {
			out = appendTableSources(out, v.Right)
		}
	}
	return
}

func (nr *nameResolver) tableUniqueName(schema, table model.CIStr) string {
	if schema.L != "" && schema.L != nr.DefaultSchema.L {
		return schema.L + "." + table.L
	}
	return table.L
}

func (nr *nameResolver) handlePosition(pos *ast.PositionExpr) {
	ctx := nr.currentContext()
	if pos.N < 1 || pos.N > len(ctx.fieldList) {
		nr.Err = errors.Errorf("Unknown column '%d'", pos.N)
		return
	}
	matched := ctx.fieldList[pos.N-1]
	nf := *matched
	expr := matched.Expr
	if cexpr, ok := expr.(*ast.ColumnNameExpr); ok {
		expr = cexpr.Refer.Expr
	}
	nf.Expr = expr
	pos.Refer = &nf
	if nr.currentContext().inGroupBy {
		// make sure item is not aggregate function
		if ast.HasAggFlag(pos.Refer.Expr) {
			nr.Err = errors.New("group by cannot contain aggregate function")
		}
	}
}

func (nr *nameResolver) handleUnionSelectList(u *ast.UnionSelectList) {
	firstSelFields := u.Selects[0].GetResultFields()
	unionFields := make([]*ast.ResultField, len(firstSelFields))
	// Copy first result fields, because we may change the result field type.
	for i, v := range firstSelFields {
		rf := *v
		col := *v.Column
		rf.Column = &col
		if rf.Column.Flen == 0 {
			rf.Column.Flen = types.UnspecifiedLength
		}
		rf.Expr = &ast.ValueExpr{}
		unionFields[i] = &rf
	}
	nr.currentContext().fieldList = unionFields
}

func (nr *nameResolver) fillShowFields(s *ast.ShowStmt) {
	if s.DBName == "" {
		if s.Table != nil && s.Table.Schema.L != "" {
			s.DBName = s.Table.Schema.O
		} else {
			s.DBName = nr.DefaultSchema.O
		}
	} else if s.Table != nil && s.Table.Schema.L == "" {
		s.Table.Schema = model.NewCIStr(s.DBName)
	}
	var fields []*ast.ResultField
	var (
		names  []string
		ftypes []byte
	)
	switch s.Tp {
	case ast.ShowEngines:
		names = []string{"Engine", "Support", "Comment", "Transactions", "XA", "Savepoints"}
	case ast.ShowDatabases:
		names = []string{"Database"}
	case ast.ShowTables:
		names = []string{fmt.Sprintf("Tables_in_%s", s.DBName)}
		if s.Full {
			names = append(names, "Table_type")
		}
	case ast.ShowTableStatus:
		names = []string{"Name", "Engine", "Version", "Row_format", "Rows", "Avg_row_length",
			"Data_length", "Max_data_length", "Index_length", "Data_free", "Auto_increment",
			"Create_time", "Update_time", "Check_time", "Collation", "Checksum",
			"Create_options", "Comment"}
		ftypes = []byte{mysql.TypeVarchar, mysql.TypeVarchar, mysql.TypeLonglong, mysql.TypeVarchar, mysql.TypeLonglong, mysql.TypeLonglong,
			mysql.TypeLonglong, mysql.TypeLonglong, mysql.TypeLonglong, mysql.TypeLonglong, mysql.TypeLonglong,
			mysql.TypeDatetime, mysql.TypeDatetime, mysql.TypeDatetime, mysql.TypeVarchar, mysql.TypeVarchar,
			mysql.TypeVarchar, mysql.TypeVarchar}
	case ast.ShowColumns:
		names = column.ColDescFieldNames(s.Full)
	case ast.ShowWarnings:
		names = []string{"Level", "Code", "Message"}
		ftypes = []byte{mysql.TypeVarchar, mysql.TypeLong, mysql.TypeVarchar}
	case ast.ShowCharset:
		names = []string{"Charset", "Description", "Default collation", "Maxlen"}
		ftypes = []byte{mysql.TypeVarchar, mysql.TypeVarchar, mysql.TypeVarchar, mysql.TypeLonglong}
	case ast.ShowVariables:
		names = []string{"Variable_name", "Value"}
	case ast.ShowStatus:
		names = []string{"Variable_name", "Value"}
	case ast.ShowCollation:
		names = []string{"Collation", "Charset", "Id", "Default", "Compiled", "Sortlen"}
		ftypes = []byte{mysql.TypeVarchar, mysql.TypeVarchar, mysql.TypeLonglong,
			mysql.TypeVarchar, mysql.TypeVarchar, mysql.TypeLonglong}
	case ast.ShowCreateTable:
		names = []string{"Table", "Create Table"}
	case ast.ShowGrants:
		names = []string{fmt.Sprintf("Grants for %s", s.User)}
	case ast.ShowTriggers:
		names = []string{"Trigger", "Event", "Table", "Statement", "Timing", "Created",
			"sql_mode", "Definer", "character_set_client", "collation_connection", "Database Collation"}
		ftypes = []byte{mysql.TypeVarchar, mysql.TypeVarchar, mysql.TypeVarchar, mysql.TypeVarchar, mysql.TypeVarchar, mysql.TypeVarchar,
			mysql.TypeVarchar, mysql.TypeVarchar, mysql.TypeVarchar, mysql.TypeVarchar, mysql.TypeVarchar}
	case ast.ShowProcedureStatus:
		names = []string{}
		ftypes = []byte{}
	case ast.ShowIndex:
		names = []string{"Table", "Non_unique", "Key_name", "Seq_in_index",
			"Column_name", "Collation", "Cardinality", "Sub_part", "Packed",
			"Null", "Index_type", "Comment", "Index_comment"}
		ftypes = []byte{mysql.TypeVarchar, mysql.TypeLonglong, mysql.TypeVarchar, mysql.TypeLonglong,
			mysql.TypeVarchar, mysql.TypeVarchar, mysql.TypeLonglong, mysql.TypeLonglong,
			mysql.TypeVarchar, mysql.TypeVarchar, mysql.TypeVarchar, mysql.TypeVarchar, mysql.TypeVarchar}
	}
	for i, name := range names {
		f := &ast.ResultField{
			ColumnAsName: model.NewCIStr(name),
			Column:       &model.ColumnInfo{}, // Empty column info.
			Table:        &model.TableInfo{},  // Empty table info.
		}
		if ftypes == nil || ftypes[i] == 0 {
			// use varchar as the default return column type
			f.Column.Tp = mysql.TypeVarchar
		} else {
			f.Column.Tp = ftypes[i]
		}
		f.Column.Charset, f.Column.Collate = types.DefaultCharsetForType(f.Column.Tp)
		f.Expr = &ast.ValueExpr{}
		f.Expr.SetType(&f.Column.FieldType)
		fields = append(fields, f)
	}

	if s.Pattern != nil && s.Pattern.Expr == nil {
		rf := fields[0]
		s.Pattern.Expr = &ast.ColumnNameExpr{
			Name: &ast.ColumnName{Name: rf.ColumnAsName},
		}
		ast.SetFlag(s.Pattern)
	}
	s.SetResultFields(fields)
	nr.currentContext().fieldList = fields
}
