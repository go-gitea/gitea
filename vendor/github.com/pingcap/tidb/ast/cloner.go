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

package ast

import "fmt"

// Cloner is an ast visitor that clones a node.
type Cloner struct {
}

// Enter implements Visitor Enter interface.
func (c *Cloner) Enter(node Node) (Node, bool) {
	return copyStruct(node), false
}

// Leave implements Visitor Leave interface.
func (c *Cloner) Leave(in Node) (out Node, ok bool) {
	return in, true
}

// copyStruct copies a node's struct value, if the struct has slice member,
// make a new slice and copy old slice value to new slice.
func copyStruct(in Node) (out Node) {
	switch v := in.(type) {
	case *ValueExpr:
		nv := *v
		out = &nv
	case *BetweenExpr:
		nv := *v
		out = &nv
	case *BinaryOperationExpr:
		nv := *v
		out = &nv
	case *WhenClause:
		nv := *v
		out = &nv
	case *CaseExpr:
		nv := *v
		nv.WhenClauses = make([]*WhenClause, len(v.WhenClauses))
		copy(nv.WhenClauses, v.WhenClauses)
		out = &nv
	case *SubqueryExpr:
		nv := *v
		out = &nv
	case *CompareSubqueryExpr:
		nv := *v
		out = &nv
	case *ColumnName:
		nv := *v
		out = &nv
	case *ColumnNameExpr:
		nv := *v
		out = &nv
	case *DefaultExpr:
		nv := *v
		out = &nv
	case *ExistsSubqueryExpr:
		nv := *v
		out = &nv
	case *PatternInExpr:
		nv := *v
		nv.List = make([]ExprNode, len(v.List))
		copy(nv.List, v.List)
		out = &nv
	case *IsNullExpr:
		nv := *v
		out = &nv
	case *IsTruthExpr:
		nv := *v
		out = &nv
	case *PatternLikeExpr:
		nv := *v
		out = &nv
	case *ParamMarkerExpr:
		nv := *v
		out = &nv
	case *ParenthesesExpr:
		nv := *v
		out = &nv
	case *PositionExpr:
		nv := *v
		out = &nv
	case *PatternRegexpExpr:
		nv := *v
		out = &nv
	case *RowExpr:
		nv := *v
		nv.Values = make([]ExprNode, len(v.Values))
		copy(nv.Values, v.Values)
		out = &nv
	case *UnaryOperationExpr:
		nv := *v
		out = &nv
	case *ValuesExpr:
		nv := *v
		out = &nv
	case *VariableExpr:
		nv := *v
		out = &nv
	case *Join:
		nv := *v
		out = &nv
	case *TableName:
		nv := *v
		out = &nv
	case *TableSource:
		nv := *v
		out = &nv
	case *OnCondition:
		nv := *v
		out = &nv
	case *WildCardField:
		nv := *v
		out = &nv
	case *SelectField:
		nv := *v
		out = &nv
	case *FieldList:
		nv := *v
		nv.Fields = make([]*SelectField, len(v.Fields))
		copy(nv.Fields, v.Fields)
		out = &nv
	case *TableRefsClause:
		nv := *v
		out = &nv
	case *ByItem:
		nv := *v
		out = &nv
	case *GroupByClause:
		nv := *v
		nv.Items = make([]*ByItem, len(v.Items))
		copy(nv.Items, v.Items)
		out = &nv
	case *HavingClause:
		nv := *v
		out = &nv
	case *OrderByClause:
		nv := *v
		nv.Items = make([]*ByItem, len(v.Items))
		copy(nv.Items, v.Items)
		out = &nv
	case *SelectStmt:
		nv := *v
		out = &nv
	case *UnionSelectList:
		nv := *v
		nv.Selects = make([]*SelectStmt, len(v.Selects))
		copy(nv.Selects, v.Selects)
		out = &nv
	case *UnionStmt:
		nv := *v
		out = &nv
	default:
		// We currently only handle expression and select statement.
		// Will add more when we need to.
		panic("unknown ast Node type " + fmt.Sprintf("%T", v))
	}
	return
}
