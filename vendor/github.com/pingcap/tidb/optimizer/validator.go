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
	"github.com/juju/errors"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/parser"
	"github.com/pingcap/tidb/parser/opcode"
)

// Validate checkes whether the node is valid.
func Validate(node ast.Node, inPrepare bool) error {
	v := validator{inPrepare: inPrepare}
	node.Accept(&v)
	return v.err
}

// validator is an ast.Visitor that validates
// ast Nodes parsed from parser.
type validator struct {
	err           error
	wildCardCount int
	inPrepare     bool
	inAggregate   bool
}

func (v *validator) Enter(in ast.Node) (out ast.Node, skipChildren bool) {
	switch in.(type) {
	case *ast.AggregateFuncExpr:
		if v.inAggregate {
			// Aggregate function can not contain aggregate function.
			v.err = ErrInvalidGroupFuncUse
			return in, true
		}
		v.inAggregate = true
	}
	return in, false
}

func (v *validator) Leave(in ast.Node) (out ast.Node, ok bool) {
	switch x := in.(type) {
	case *ast.AggregateFuncExpr:
		v.inAggregate = false
	case *ast.BetweenExpr:
		v.checkAllOneColumn(x.Expr, x.Left, x.Right)
	case *ast.BinaryOperationExpr:
		v.checkBinaryOperation(x)
	case *ast.ByItem:
		v.checkAllOneColumn(x.Expr)
	case *ast.CreateTableStmt:
		v.checkAutoIncrement(x)
	case *ast.CompareSubqueryExpr:
		v.checkSameColumns(x.L, x.R)
	case *ast.FieldList:
		v.checkFieldList(x)
	case *ast.HavingClause:
		v.checkAllOneColumn(x.Expr)
	case *ast.IsNullExpr:
		v.checkAllOneColumn(x.Expr)
	case *ast.IsTruthExpr:
		v.checkAllOneColumn(x.Expr)
	case *ast.ParamMarkerExpr:
		if !v.inPrepare {
			v.err = parser.ErrSyntax.Gen("syntax error, unexpected '?'")
		}
	case *ast.PatternInExpr:
		v.checkSameColumns(append(x.List, x.Expr)...)
	}

	return in, v.err == nil
}

// checkAllOneColumn checks that all expressions have one column.
// Expression may have more than one column when it is a rowExpr or
// a Subquery with more than one result fields.
func (v *validator) checkAllOneColumn(exprs ...ast.ExprNode) {
	for _, expr := range exprs {
		switch x := expr.(type) {
		case *ast.RowExpr:
			v.err = ErrOneColumn
		case *ast.SubqueryExpr:
			if len(x.Query.GetResultFields()) != 1 {
				v.err = ErrOneColumn
			}
		}
	}
	return
}

func checkAutoIncrementOp(colDef *ast.ColumnDef, num int) (bool, error) {
	var hasAutoIncrement bool

	if colDef.Options[num].Tp == ast.ColumnOptionAutoIncrement {
		hasAutoIncrement = true
		if len(colDef.Options) == num+1 {
			return hasAutoIncrement, nil
		}
		for _, op := range colDef.Options[num+1:] {
			if op.Tp == ast.ColumnOptionDefaultValue {
				return hasAutoIncrement, errors.Errorf("Invalid default value for '%s'", colDef.Name.Name.O)
			}
		}
	}
	if colDef.Options[num].Tp == ast.ColumnOptionDefaultValue && len(colDef.Options) != num+1 {
		for _, op := range colDef.Options[num+1:] {
			if op.Tp == ast.ColumnOptionAutoIncrement {
				return hasAutoIncrement, errors.Errorf("Invalid default value for '%s'", colDef.Name.Name.O)
			}
		}
	}

	return hasAutoIncrement, nil
}

func isConstraintKeyTp(constraints []*ast.Constraint, colDef *ast.ColumnDef) bool {
	for _, c := range constraints {
		if len(c.Keys) < 1 {
		}
		// If the constraint as follows: primary key(c1, c2)
		// we only support c1 column can be auto_increment.
		if colDef.Name.Name.L != c.Keys[0].Column.Name.L {
			continue
		}
		switch c.Tp {
		case ast.ConstraintPrimaryKey, ast.ConstraintKey, ast.ConstraintIndex,
			ast.ConstraintUniq, ast.ConstraintUniqIndex, ast.ConstraintUniqKey:
			return true
		}
	}

	return false
}

func (v *validator) checkAutoIncrement(stmt *ast.CreateTableStmt) {
	var (
		isKey            bool
		count            int
		autoIncrementCol *ast.ColumnDef
	)

	for _, colDef := range stmt.Cols {
		var hasAutoIncrement bool
		for i, op := range colDef.Options {
			ok, err := checkAutoIncrementOp(colDef, i)
			if err != nil {
				v.err = err
				return
			}
			if ok {
				hasAutoIncrement = true
			}
			switch op.Tp {
			case ast.ColumnOptionPrimaryKey, ast.ColumnOptionUniqKey, ast.ColumnOptionUniqIndex,
				ast.ColumnOptionUniq, ast.ColumnOptionKey, ast.ColumnOptionIndex:
				isKey = true
			}
		}
		if hasAutoIncrement {
			count++
			autoIncrementCol = colDef
		}
	}

	if count < 1 {
		return
	}

	if !isKey {
		isKey = isConstraintKeyTp(stmt.Constraints, autoIncrementCol)
	}
	if !isKey || count > 1 {
		v.err = errors.New("Incorrect table definition; there can be only one auto column and it must be defined as a key")
	}

	switch autoIncrementCol.Tp.Tp {
	case mysql.TypeTiny, mysql.TypeShort, mysql.TypeLong,
		mysql.TypeFloat, mysql.TypeDouble, mysql.TypeLonglong, mysql.TypeInt24:
	default:
		v.err = errors.Errorf("Incorrect column specifier for column '%s'", autoIncrementCol.Name.Name.O)
	}
}

func (v *validator) checkBinaryOperation(x *ast.BinaryOperationExpr) {
	// row constructor only supports comparison operation.
	switch x.Op {
	case opcode.LT, opcode.LE, opcode.GE, opcode.GT, opcode.EQ, opcode.NE, opcode.NullEQ:
		v.checkSameColumns(x.L, x.R)
	default:
		v.checkAllOneColumn(x.L, x.R)
	}
}

func columnCount(ex ast.ExprNode) int {
	switch x := ex.(type) {
	case *ast.RowExpr:
		return len(x.Values)
	case *ast.SubqueryExpr:
		return len(x.Query.GetResultFields())
	default:
		return 1
	}
}

func (v *validator) checkSameColumns(exprs ...ast.ExprNode) {
	if len(exprs) == 0 {
		return
	}
	count := columnCount(exprs[0])
	for i := 1; i < len(exprs); i++ {
		if columnCount(exprs[i]) != count {
			v.err = ErrSameColumns
			return
		}
	}
}

// checkFieldList checks if there is only one '*' and each field has only one column.
func (v *validator) checkFieldList(x *ast.FieldList) {
	var hasWildCard bool
	for _, val := range x.Fields {
		if val.WildCard != nil && val.WildCard.Table.L == "" {
			if hasWildCard {
				v.err = ErrMultiWildCard
				return
			}
			hasWildCard = true
		}
		v.checkAllOneColumn(val.Expr)
		if v.err != nil {
			return
		}
	}
}
